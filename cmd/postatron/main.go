// Command postatron is the Postatron CLI: six commands, one per public API
// endpoint. Authenticate with --api-key or POSTATRON_API_KEY.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/henrwal/postatron-cli/apiv1"
	"github.com/spf13/cobra"
)

// version is overridden at release time with -ldflags "-X main.version=v1.2.3".
var version = "1.0.0"

func main() {
	root, _ := newRootCommand(os.Stdout, os.Stderr)
	if err := root.Execute(); err != nil {
		os.Exit(exitCode(err))
	}
}

type globals struct {
	apiKey  string
	baseURL string
	asJSON  bool
	out     io.Writer
	client  apiv1.Operations
}

func (g *globals) ops() apiv1.Operations {
	if g.client != nil {
		return g.client
	}
	key := g.apiKey
	if key == "" {
		key = os.Getenv("POSTATRON_API_KEY")
	}
	base := g.baseURL
	if base == "" {
		base = os.Getenv("POSTATRON_API_URL")
	}
	client := apiv1.NewClient(base, key)
	client.UserAgent = "postatron-cli/" + version
	return client
}

// newRootCommand builds the CLI. The returned globals let tests inject a fake
// Operations implementation.
func newRootCommand(stdout, stderr io.Writer) (*cobra.Command, *globals) {
	g := &globals{out: stdout}

	root := &cobra.Command{
		Use:           "postatron",
		Short:         "Create, schedule and manage Postatron posts from the terminal",
		Long:          "Postatron CLI. Six commands, one per API endpoint. Set POSTATRON_API_KEY (or --api-key) with a key from https://postatron.com/dashboard/api.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&g.apiKey, "api-key", "", "API key (defaults to $POSTATRON_API_KEY)")
	root.PersistentFlags().StringVar(&g.baseURL, "api-url", "", "API base URL (defaults to $POSTATRON_API_URL or https://api.postatron.com)")
	root.PersistentFlags().BoolVar(&g.asJSON, "json", false, "print raw JSON responses")

	root.AddCommand(
		newCreatePostCommand(g),
		newListPostsCommand(g),
		newGetPostCommand(g),
		newDeletePostCommand(g),
		newListAccountsCommand(g),
		newGetUsageCommand(g),
	)
	return root, g
}

func newCreatePostCommand(g *globals) *cobra.Command {
	var (
		content     string
		platforms   []string
		accountIDs  []string
		scheduledAt string
	)
	cmd := &cobra.Command{
		Use:   "create-post",
		Short: "Create a post (publishes now, or at --scheduled-at)",
		Example: `  postatron create-post --content "Launch day!" --platforms x,linkedin
  postatron create-post --content "Tomorrow 9am" --platforms bluesky --scheduled-at 2026-09-10T09:00:00Z
  postatron create-post --content "Only this account" --account-ids 1234567890`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := apiv1.CreatePostRequest{Content: content, Platforms: platforms, AccountIDs: accountIDs}
			if scheduledAt != "" {
				when, err := time.Parse(time.RFC3339, scheduledAt)
				if err != nil {
					return usageError("--scheduled-at must be RFC 3339, e.g. 2026-09-10T09:00:00Z")
				}
				req.ScheduledAt = &when
			}
			post, err := g.ops().CreatePost(cmd.Context(), req)
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, post)
			}
			printPost(g.out, *post)
			return nil
		},
	}
	cmd.Flags().StringVarP(&content, "content", "c", "", "post text (required)")
	cmd.Flags().StringSliceVarP(&platforms, "platforms", "p", nil, "platforms to post to, e.g. x,linkedin (uses every connected account on each)")
	cmd.Flags().StringSliceVarP(&accountIDs, "account-ids", "a", nil, "specific account ids from list-accounts")
	cmd.Flags().StringVar(&scheduledAt, "scheduled-at", "", "RFC 3339 time to schedule, at least 5 minutes ahead")
	_ = cmd.MarkFlagRequired("content")
	return cmd
}

func newListPostsCommand(g *globals) *cobra.Command {
	var (
		status, platform, from, to, cursor string
		limit                              int
	)
	cmd := &cobra.Command{
		Use:   "list-posts",
		Short: "List posts, newest first",
		Example: `  postatron list-posts --status scheduled
  postatron list-posts --platform x --from 2026-09-01T00:00:00Z --to 2026-09-30T23:59:59Z`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			query := apiv1.ListPostsQuery{Status: status, Platform: platform, Limit: limit, Cursor: cursor}
			if from != "" {
				t, err := time.Parse(time.RFC3339, from)
				if err != nil {
					return usageError("--from must be RFC 3339")
				}
				query.From = &t
			}
			if to != "" {
				t, err := time.Parse(time.RFC3339, to)
				if err != nil {
					return usageError("--to must be RFC 3339")
				}
				query.To = &t
			}
			list, err := g.ops().ListPosts(cmd.Context(), query)
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, list)
			}
			printPostList(g.out, *list)
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "scheduled, pending, processing, published, failed or partial")
	cmd.Flags().StringVar(&platform, "platform", "", "only posts targeting this platform")
	cmd.Flags().StringVar(&from, "from", "", "RFC 3339 start of range")
	cmd.Flags().StringVar(&to, "to", "", "RFC 3339 end of range")
	cmd.Flags().IntVar(&limit, "limit", 0, "page size (1-100, default 25)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "next_cursor from the previous page")
	return cmd
}

func newGetPostCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "get-post <id>",
		Short: "Show a post and its per-platform delivery status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			post, err := g.ops().GetPost(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, post)
			}
			printPost(g.out, *post)
			return nil
		},
	}
}

func newDeletePostCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "delete-post <id>",
		Short: "Cancel a scheduled post (no-op once published)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := g.ops().DeletePost(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, out)
			}
			if out.Cancelled {
				fmt.Fprintf(g.out, "Cancelled %s\n", out.ID)
			} else {
				fmt.Fprintf(g.out, "Not cancelled: %s is %s. %s\n", out.ID, out.Status, out.Message)
			}
			return nil
		},
	}
}

func newListAccountsCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "list-accounts",
		Short: "List connected social accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			list, err := g.ops().ListAccounts(cmd.Context())
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, list)
			}
			tw := tabwriter.NewWriter(g.out, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tPLATFORM\tUSERNAME\tSTATUS\tCONNECTED")
			for _, a := range list.Data {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", a.ID, a.Platform, a.Username, a.Status, a.ConnectedAt.Format("2006-01-02"))
			}
			return tw.Flush()
		},
	}
}

func newGetUsageCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "get-usage",
		Short: "Show this month's quota and posts per platform",
		RunE: func(cmd *cobra.Command, _ []string) error {
			report, err := g.ops().GetUsage(cmd.Context())
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, report)
			}
			printUsage(g.out, *report)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// output
// ---------------------------------------------------------------------------

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printPost(w io.Writer, p apiv1.Post) {
	fmt.Fprintf(w, "Post %s  [%s]\n", p.ID, p.Status)
	if p.ScheduledAt != nil {
		fmt.Fprintf(w, "Scheduled: %s\n", p.ScheduledAt.UTC().Format(time.RFC3339))
	}
	fmt.Fprintf(w, "Created:   %s\n", p.CreatedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(w, "Content:   %s\n", oneLine(p.Content))
	if len(p.Deliveries) > 0 {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "PLATFORM\tACCOUNT\tSTATUS\tURL / ERROR")
		for _, d := range p.Deliveries {
			detail := d.URL
			if d.Error != "" {
				detail = d.Error
			}
			fmt.Fprintf(tw, "%s\t@%s\t%s\t%s\n", d.Platform, d.Username, d.Status, detail)
		}
		_ = tw.Flush()
	}
}

func printPostList(w io.Writer, list apiv1.PostList) {
	if len(list.Data) == 0 {
		fmt.Fprintln(w, "No posts found.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tWHEN\tPLATFORMS\tCONTENT")
	for _, p := range list.Data {
		when := p.CreatedAt
		if p.ScheduledAt != nil {
			when = *p.ScheduledAt
		}
		platforms := make([]string, 0, len(p.Deliveries))
		for _, d := range p.Deliveries {
			platforms = append(platforms, d.Platform)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.ID, p.Status, when.UTC().Format("2006-01-02 15:04"), strings.Join(platforms, ","), truncate(oneLine(p.Content), 60))
	}
	_ = tw.Flush()
	if list.NextCursor != "" {
		fmt.Fprintf(w, "\nMore results: --cursor %s\n", list.NextCursor)
	}
}

func printUsage(w io.Writer, r apiv1.UsageReport) {
	fmt.Fprintf(w, "Plan: %s (%s, %s)  Period: %s to %s\n", r.Plan.Name, r.Plan.Interval, strings.ToLower(r.Plan.Status),
		r.Period.Start.Format("2006-01-02"), r.Period.End.Add(-time.Second).Format("2006-01-02"))
	fmt.Fprintf(w, "Destination-posts: %d / %d (%d remaining)\n", r.DestinationPosts.Used, r.DestinationPosts.Limit, r.DestinationPosts.Remaining)
	fmt.Fprintf(w, "X link posts:      %d / %d (%d remaining)\n", r.XLinkPosts.Used, r.XLinkPosts.Limit, r.XLinkPosts.Remaining)
	fmt.Fprintf(w, "Link posts (all):  %d\n", r.LinkPosts.Used)
	fmt.Fprintf(w, "Accounts:          %d / %d\n", r.Plan.ConnectedAccounts, r.Plan.MaxConnectedAccounts)
	if r.Overage.Enabled {
		fmt.Fprintf(w, "Overage:           %d destination-posts, %d X link posts (at $%.2f / $%.2f each)\n",
			r.Overage.DestinationPosts, r.Overage.XLinkPosts, r.Overage.DestinationPostUnitPriceUSD, r.Overage.XLinkPostUnitPriceUSD)
	}
	fmt.Fprintf(w, "Rate limits:       %d writes/hour, %d reads/minute\n", r.RateLimits.WritesPerHour, r.RateLimits.ReadsPerMinute)

	if len(r.ByPlatform) > 0 {
		platforms := make([]string, 0, len(r.ByPlatform))
		for p := range r.ByPlatform {
			platforms = append(platforms, p)
		}
		sort.Strings(platforms)
		fmt.Fprintln(w, "\nPosts per platform:")
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		for _, p := range platforms {
			fmt.Fprintf(tw, "  %s\t%d\n", p, r.ByPlatform[p])
		}
		_ = tw.Flush()
	}
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// ---------------------------------------------------------------------------
// errors
// ---------------------------------------------------------------------------

type usageErr struct{ msg string }

func (e usageErr) Error() string { return e.msg }

func usageError(msg string) error { return usageErr{msg: msg} }

// exitCode maps errors to exit statuses: 2 for usage, 3 for auth, 4 for
// quota/rate limits, 1 otherwise. The message is printed to stderr.
func exitCode(err error) int {
	fmt.Fprintf(os.Stderr, "postatron: %v\n", err)
	var uerr usageErr
	if errors.As(err, &uerr) {
		return 2
	}
	var apiErr *apiv1.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case apiv1.CodeUnauthorized, apiv1.CodeInsufficientScope, apiv1.CodeSubscriptionRequired:
			return 3
		case apiv1.CodeRateLimited, apiv1.CodeQuotaExceeded:
			return 4
		}
	}
	if strings.Contains(err.Error(), "no API key configured") {
		return 3
	}
	return 1
}

var _ = context.Background
