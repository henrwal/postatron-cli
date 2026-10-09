// Command postatron is the Postatron CLI: one command per public API
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
		Long:          "Postatron CLI. One command per API endpoint. Set POSTATRON_API_KEY (or --api-key) with a key from https://postatron.com/dashboard/api.",
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
		newDeletePostsCommand(g),
		newUpdatePostCommand(g),
		newListAccountsCommand(g),
		newConnectAccountCommand(g),
		newListProfilesCommand(g),
		newListQueuesCommand(g),
		newCreateQueueCommand(g),
		newUpdateQueueCommand(g),
		newDeleteQueueCommand(g),
		newGetUsageCommand(g),
		newGetAnalyticsCommand(g),
		newCreateUploadCommand(g),
		newGetUploadCommand(g),
	)
	return root, g
}

func newCreatePostCommand(g *globals) *cobra.Command {
	var (
		content     string
		platforms   []string
		accountIDs  []string
		profile     string
		mediaURLs   []string
		mediaIDs    []string
		scheduledAt string
		queue       string
		options     platformFlags
	)
	cmd := &cobra.Command{
		Use:   "create-post",
		Short: "Create a post (publishes now, at --scheduled-at, or in a --queue's next slot)",
		Example: `  postatron create-post --content "Launch day!" --platforms x,linkedin
  postatron create-post --content "Tomorrow 9am" --platforms bluesky --scheduled-at 2026-09-10T09:00:00Z
  postatron create-post --content "Only this account" --account-ids 1234567890
  postatron create-post --content "For the Acme profile" --platforms x,instagram --profile Acme --media-urls https://example.com/launch.jpg
  postatron create-post --content "Whenever there's room" --platforms x,linkedin --queue "Weekday mornings"
  postatron create-post --content "Tea or coffee?" --platforms x --poll "Tea,Coffee" --reply-settings following
  postatron create-post --content "1/ A thread" --platforms x --thread "2/ More" --thread "3/ The end"
  postatron create-post --content "New reel" --platforms instagram --media-urls https://example.com/r.mp4 --options '{"instagram":{"post_type":"reel","trial_reel":"manual"}}'`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := apiv1.CreatePostRequest{
				Content:    content,
				Platforms:  platforms,
				AccountIDs: accountIDs,
				Profile:    profile,
				MediaURLs:  mediaURLs,
				MediaIDs:   mediaIDs,
				Queue:      strings.TrimSpace(queue),
			}
			var err error
			if req.X, req.Instagram, req.TikTok, err = options.build(); err != nil {
				return err
			}
			if scheduledAt != "" && queue != "" {
				return usageError("use --scheduled-at or --queue, not both")
			}
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
	cmd.Flags().StringSliceVarP(&platforms, "platforms", "p", nil, "platforms to post to, e.g. x,linkedin (the account on each, in --profile)")
	cmd.Flags().StringSliceVarP(&accountIDs, "account-ids", "a", nil, "specific account ids from list-accounts")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name or id from list-profiles; needed when a platform has accounts in several profiles")
	cmd.Flags().StringSliceVar(&mediaURLs, "media-urls", nil, "public https links to images or a video to attach (up to 4)")
	cmd.Flags().StringSliceVar(&mediaIDs, "media-ids", nil, "upload ids to attach")
	cmd.Flags().StringVar(&scheduledAt, "scheduled-at", "", "RFC 3339 time to schedule, at least 5 minutes ahead")
	cmd.Flags().StringVar(&queue, "queue", "", "queue name or id from list-queues: post in its next free slot")
	options.register(cmd)
	_ = cmd.MarkFlagRequired("content")
	return cmd
}

// platformFlags are the per-platform options. The common ones have flags of
// their own; --options takes the whole set as JSON for everything else.
type platformFlags struct {
	raw           string
	thread        []string
	poll          []string
	pollMinutes   int
	replySettings string
	community     string
	igType        string
	firstComment  string
}

func (f *platformFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.raw, "options", "", `platform options as JSON, e.g. '{"x":{...},"instagram":{...},"tiktok":{...}}', or @file.json`)
	cmd.Flags().StringArrayVar(&f.thread, "thread", nil, "an X reply under the first tweet; repeat for each, in order")
	cmd.Flags().StringSliceVar(&f.poll, "poll", nil, "make the X post a poll with these 2 to 4 options")
	cmd.Flags().IntVar(&f.pollMinutes, "poll-minutes", 0, "how long the X poll runs (5 to 10080, default a day)")
	cmd.Flags().StringVar(&f.replySettings, "reply-settings", "", "who can reply on X: following, mentioned_users, subscribers or verified")
	cmd.Flags().StringVar(&f.community, "community", "", "X Community id or link to post into")
	cmd.Flags().StringVar(&f.igType, "instagram-type", "", "Instagram post type: auto, feed, story, reel or carousel")
	cmd.Flags().StringVar(&f.firstComment, "first-comment", "", "Instagram first comment")
}

// options is the shape --options takes.
type options struct {
	X         *apiv1.XOptions         `json:"x,omitempty"`
	Instagram *apiv1.InstagramOptions `json:"instagram,omitempty"`
	TikTok    *apiv1.TikTokOptions    `json:"tiktok,omitempty"`
}

// build merges --options with the individual flags, which win.
func (f *platformFlags) build() (*apiv1.XOptions, *apiv1.InstagramOptions, *apiv1.TikTokOptions, error) {
	var o options
	if raw := strings.TrimSpace(f.raw); raw != "" {
		data := []byte(raw)
		if strings.HasPrefix(raw, "@") {
			read, err := os.ReadFile(strings.TrimPrefix(raw, "@"))
			if err != nil {
				return nil, nil, nil, usageError("--options: " + err.Error())
			}
			data = read
		}
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&o); err != nil {
			return nil, nil, nil, usageError("--options is not valid: " + err.Error())
		}
	}
	x := func() *apiv1.XOptions {
		if o.X == nil {
			o.X = &apiv1.XOptions{}
		}
		return o.X
	}
	for _, tweet := range f.thread {
		x().Thread = append(x().Thread, apiv1.XThreadTweet{Content: tweet})
	}
	if len(f.poll) > 0 {
		x().Poll = &apiv1.XPoll{Options: f.poll, DurationMinutes: f.pollMinutes}
	}
	if f.replySettings != "" {
		x().ReplySettings = f.replySettings
	}
	if f.community != "" {
		x().Community = f.community
	}
	if f.igType != "" || f.firstComment != "" {
		if o.Instagram == nil {
			o.Instagram = &apiv1.InstagramOptions{}
		}
		if f.igType != "" {
			o.Instagram.PostType = f.igType
		}
		if f.firstComment != "" {
			o.Instagram.FirstComment = f.firstComment
		}
	}
	return o.X, o.Instagram, o.TikTok, nil
}

func newListPostsCommand(g *globals) *cobra.Command {
	var (
		status, platform, profile, from, to, cursor string
		limit                                       int
	)
	cmd := &cobra.Command{
		Use:   "list-posts",
		Short: "List posts, newest first",
		Example: `  postatron list-posts --status scheduled
  postatron list-posts --platform x --from 2026-09-01T00:00:00Z --to 2026-09-30T23:59:59Z`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			query := apiv1.ListPostsQuery{Status: status, Platform: platform, Profile: profile, Limit: limit, Cursor: cursor}
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
	cmd.Flags().StringVar(&status, "status", "", "draft, scheduled, pending, processing, published, failed or partial")
	cmd.Flags().StringVar(&platform, "platform", "", "only posts targeting this platform")
	cmd.Flags().StringVar(&profile, "profile", "", "only posts to accounts in this profile (name or id)")
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
		Short: "Delete a post (a published one is only removed from Postatron, not the platforms)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := g.ops().DeletePost(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, out)
			}
			switch {
			case out.Cancelled:
				fmt.Fprintf(g.out, "Cancelled %s\n", out.ID)
			case out.Deleted:
				fmt.Fprintf(g.out, "Deleted %s. %s\n", out.ID, out.Message)
			default:
				fmt.Fprintf(g.out, "Not deleted: %s is %s. %s\n", out.ID, out.Status, out.Message)
			}
			return nil
		},
	}
}

func newDeletePostsCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "delete-posts <id> [<id>...]",
		Short: "Delete up to 100 posts at once (published ones are only removed from Postatron)",
		Args:  cobra.RangeArgs(1, apiv1.MaxBulkDelete),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := g.ops().DeletePosts(cmd.Context(), apiv1.DeletePostsRequest{IDs: args})
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, out)
			}
			fmt.Fprintf(g.out, "Deleted %d of %d.\n", len(out.Deleted), len(args))
			for _, f := range out.Failed {
				fmt.Fprintf(g.out, "  %s: %s\n", f.ID, f.Error)
			}
			if out.Message != "" {
				fmt.Fprintln(g.out, out.Message)
			}
			return nil
		},
	}
}

func newUpdatePostCommand(g *globals) *cobra.Command {
	var (
		content, scheduledAt, profile         string
		addPlatforms, addAccounts, removeAccs []string
		publishNow                            bool
		options                               platformFlags
	)
	cmd := &cobra.Command{
		Use:   "update-post <id>",
		Short: "Change a draft or scheduled post, or schedule or publish a draft",
		Example: `  postatron update-post p_123 --add-platforms instagram
  postatron update-post p_123 --scheduled-at 2026-09-10T10:00:00Z
  postatron update-post p_456 --add-platforms linkedin --publish-now
  postatron update-post p_123 --content "New wording" --remove-account-ids 1234567890`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if publishNow && scheduledAt != "" {
				return usageError("--publish-now and --scheduled-at cannot be used together")
			}
			req := apiv1.UpdatePostRequest{AddPlatforms: addPlatforms, AddAccountIDs: addAccounts, RemoveAccountIDs: removeAccs, Profile: profile, PublishNow: publishNow}
			var err error
			if req.X, req.Instagram, req.TikTok, err = options.build(); err != nil {
				return err
			}
			if cmd.Flags().Changed("content") {
				req.Content = &content
			}
			if scheduledAt != "" {
				when, err := time.Parse(time.RFC3339, scheduledAt)
				if err != nil {
					return usageError("--scheduled-at must be RFC 3339, e.g. 2026-09-10T09:00:00Z")
				}
				req.ScheduledAt = &when
			}
			post, err := g.ops().UpdatePost(cmd.Context(), args[0], req)
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
	cmd.Flags().StringVarP(&content, "content", "c", "", "new post text")
	cmd.Flags().StringVar(&scheduledAt, "scheduled-at", "", "new RFC 3339 time, at least 5 minutes ahead (schedules a draft)")
	cmd.Flags().BoolVar(&publishNow, "publish-now", false, "publish the post now, after the other changes")
	cmd.Flags().StringSliceVar(&addPlatforms, "add-platforms", nil, "platforms to add, e.g. instagram (the account on each, in --profile)")
	cmd.Flags().StringSliceVar(&addAccounts, "add-account-ids", nil, "specific account ids to add")
	cmd.Flags().StringSliceVar(&removeAccs, "remove-account-ids", nil, "account ids to take off the post")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name or id that --add-platforms means")
	options.register(cmd)
	return cmd
}

func newConnectAccountCommand(g *globals) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "connect-account <platform>",
		Short: "Get a link that connects a social account (x, instagram, linkedin, ...)",
		Example: `  postatron connect-account x
  postatron connect-account instagram --profile Acme`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			link, err := g.ops().ConnectAccount(cmd.Context(), apiv1.ConnectAccountRequest{Platform: args[0], Profile: profile})
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, link)
			}
			fmt.Fprintf(g.out, "Open this link to connect %s:\n%s\n", link.Platform, link.ConnectURL)
			if link.Hint != "" {
				fmt.Fprintln(g.out, link.Hint)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name or id to put the account in")
	return cmd
}

func newListAccountsCommand(g *globals) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "list-accounts",
		Short: "List connected social accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			list, err := g.ops().ListAccounts(cmd.Context())
			if err != nil {
				return err
			}
			if profile != "" {
				kept := list.Data[:0:0]
				for _, a := range list.Data {
					if a.ProfileID == profile || strings.EqualFold(a.ProfileName, profile) {
						kept = append(kept, a)
					}
				}
				list = &apiv1.AccountList{Data: kept}
			}
			if g.asJSON {
				return printJSON(g.out, list)
			}
			tw := tabwriter.NewWriter(g.out, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tPLATFORM\tUSERNAME\tPROFILE\tSTATUS\tCONNECTED")
			for _, a := range list.Data {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", a.ID, a.Platform, a.Username, a.ProfileName, a.Status, a.ConnectedAt.Format("2006-01-02"))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "only accounts in this profile (name or id)")
	return cmd
}

func newListProfilesCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "list-profiles",
		Short: "List profiles and the accounts in each",
		RunE: func(cmd *cobra.Command, _ []string) error {
			list, err := g.ops().ListProfiles(cmd.Context())
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, list)
			}
			tw := tabwriter.NewWriter(g.out, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tACCOUNTS")
			for _, p := range list.Data {
				accounts := make([]string, 0, len(p.Accounts))
				for _, a := range p.Accounts {
					accounts = append(accounts, a.Platform+":"+a.Username)
				}
				summary := strings.Join(accounts, ", ")
				if summary == "" {
					summary = "(none)"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", p.ID, p.Name, summary)
			}
			return tw.Flush()
		},
	}
}

func newListQueuesCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "list-queues",
		Short: "List posting queues and when each next has a free slot",
		RunE: func(cmd *cobra.Command, _ []string) error {
			list, err := g.ops().ListQueues(cmd.Context())
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, list)
			}
			tw := tabwriter.NewWriter(g.out, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tPROFILE\tSLOTS\tNEXT SLOT\tQUEUED")
			for _, q := range list.Data {
				next := q.NextSlotError
				if q.NextSlot != nil {
					next = q.NextSlot.UTC().Format(time.RFC3339)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d a week\t%s\t%d\n", q.ID, q.Name, q.ProfileName, len(q.Slots), next, q.Queued)
			}
			return tw.Flush()
		},
	}
}

// parseSlots reads slots written as "mon 09:00,wed 13:30".
func parseSlots(raw []string) ([]apiv1.QueueSlot, error) {
	days := map[string]int{"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6}
	out := make([]apiv1.QueueSlot, 0, len(raw))
	for _, entry := range raw {
		fields := strings.Fields(strings.ToLower(entry))
		if len(fields) != 2 {
			return nil, usageError(fmt.Sprintf("slot %q should look like \"mon 09:00\"", entry))
		}
		day, ok := days[fields[0][:min(3, len(fields[0]))]]
		if !ok {
			return nil, usageError(fmt.Sprintf("slot %q: %q is not a day", entry, fields[0]))
		}
		out = append(out, apiv1.QueueSlot{Day: day, Time: fields[1]})
	}
	return out, nil
}

func printQueue(w io.Writer, q apiv1.Queue) {
	state := "active"
	if !q.Active {
		state = "paused"
	}
	fmt.Fprintf(w, "Queue %s  %q  [%s]\n", q.ID, q.Name, state)
	fmt.Fprintf(w, "Profile:   %s\n", q.ProfileName)
	fmt.Fprintf(w, "Timezone:  %s\n", q.Timezone)
	fmt.Fprintf(w, "Slots:     %d a week\n", len(q.Slots))
	if q.NextSlot != nil {
		fmt.Fprintf(w, "Next slot: %s\n", q.NextSlot.UTC().Format(time.RFC3339))
	} else if q.NextSlotError != "" {
		fmt.Fprintf(w, "Next slot: %s\n", q.NextSlotError)
	}
}

func newCreateQueueCommand(g *globals) *cobra.Command {
	var (
		req   apiv1.CreateQueueRequest
		slots []string
	)
	cmd := &cobra.Command{
		Use:     "create-queue",
		Short:   "Create a posting queue: weekly time slots for one profile",
		Example: `  postatron create-queue --name "Weekday mornings" --timezone Europe/London --slots "mon 09:00,tue 09:00,wed 09:00"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			parsed, err := parseSlots(slots)
			if err != nil {
				return err
			}
			req.Slots = parsed
			q, err := g.ops().CreateQueue(cmd.Context(), req)
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, q)
			}
			printQueue(g.out, *q)
			return nil
		},
	}
	cmd.Flags().StringVar(&req.Name, "name", "", "queue name (required)")
	cmd.Flags().StringVar(&req.Timezone, "timezone", "", "IANA timezone the slots are in, e.g. Europe/London (required)")
	cmd.Flags().StringVar(&req.Profile, "profile", "", "profile name or id; Default when left out")
	cmd.Flags().StringSliceVar(&slots, "slots", nil, `weekly slots, e.g. "mon 09:00,thu 17:30"`)
	cmd.Flags().BoolVar(&req.Paused, "paused", false, "create it paused")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("timezone")
	return cmd
}

func newUpdateQueueCommand(g *globals) *cobra.Command {
	var (
		name, timezone, profile string
		slots                   []string
		pause, resume           bool
	)
	cmd := &cobra.Command{
		Use:   "update-queue <queue>",
		Short: "Change a queue's name, slots, timezone or profile, or pause or resume it",
		Example: `  postatron update-queue "Weekday mornings" --slots "mon 08:30,fri 08:30"
  postatron update-queue q_123 --pause`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if pause && resume {
				return usageError("--pause and --resume cannot be used together")
			}
			var req apiv1.UpdateQueueRequest
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("timezone") {
				req.Timezone = &timezone
			}
			if cmd.Flags().Changed("profile") {
				req.Profile = &profile
			}
			if cmd.Flags().Changed("slots") {
				parsed, err := parseSlots(slots)
				if err != nil {
					return err
				}
				req.Slots = &parsed
			}
			if pause || resume {
				active := resume
				req.Active = &active
			}
			q, err := g.ops().UpdateQueue(cmd.Context(), args[0], req)
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, q)
			}
			printQueue(g.out, *q)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&timezone, "timezone", "", "new IANA timezone")
	cmd.Flags().StringVar(&profile, "profile", "", "move the queue to this profile")
	cmd.Flags().StringSliceVar(&slots, "slots", nil, `replace every slot, e.g. "mon 09:00,thu 17:30"`)
	cmd.Flags().BoolVar(&pause, "pause", false, "stop the queue taking posts")
	cmd.Flags().BoolVar(&resume, "resume", false, "let the queue take posts again")
	return cmd
}

func newDeleteQueueCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "delete-queue <queue>",
		Short: "Delete a queue (posts it already placed stay scheduled)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := g.ops().DeleteQueue(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, out)
			}
			fmt.Fprintf(g.out, "Deleted queue %s. %s\n", out.ID, out.Message)
			return nil
		},
	}
}

// newCreateUploadCommand opens an upload slot: a link a person opens while
// signed in to Postatron to pick a file from their own device. It is how an
// agent attaches a file it cannot put on the public internet itself.
func newCreateUploadCommand(g *globals) *cobra.Command {
	var purpose string
	cmd := &cobra.Command{
		Use:     "create-upload",
		Short:   "Get a link for someone to attach a photo or video from their device",
		Example: `  postatron create-upload --purpose "photo for Tuesday's post"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			upload, err := g.ops().CreateUpload(cmd.Context(), apiv1.CreateUploadRequest{Purpose: purpose})
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, upload)
			}
			printUpload(g.out, *upload)
			return nil
		},
	}
	cmd.Flags().StringVar(&purpose, "purpose", "", "note shown on the upload page, so the person knows what to attach")
	return cmd
}

func newGetUploadCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "get-upload <id>",
		Short: "Check whether a file has arrived (PENDING, then READY)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			upload, err := g.ops().GetUpload(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, upload)
			}
			printUpload(g.out, *upload)
			return nil
		},
	}
}

func printUpload(w io.Writer, u apiv1.Upload) {
	fmt.Fprintf(w, "Upload %s  [%s]\n", u.ID, u.Status)
	if u.Filename != "" {
		fmt.Fprintf(w, "File:    %s\n", u.Filename)
	}
	if u.UploadURL != "" {
		fmt.Fprintf(w, "Link:    %s\n", u.UploadURL)
	}
	fmt.Fprintf(w, "Expires: %s\n", u.ExpiresAt)
	if u.Hint != "" {
		fmt.Fprintf(w, "%s\n", u.Hint)
	}
}

func newGetAnalyticsCommand(g *globals) *cobra.Command {
	var query apiv1.AnalyticsQuery
	cmd := &cobra.Command{
		Use:   "get-analytics",
		Short: "Show post performance and audience for a 7, 30 or 90 day window",
		Example: `  postatron get-analytics --range 7d
  postatron get-analytics --profile Acme --platform instagram`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			report, err := g.ops().GetAnalytics(cmd.Context(), query)
			if err != nil {
				return err
			}
			if g.asJSON {
				return printJSON(g.out, report)
			}
			printAnalytics(g.out, *report)
			return nil
		},
	}
	cmd.Flags().StringVar(&query.Range, "range", "", "7d, 30d or 90d (default 30d)")
	cmd.Flags().StringVar(&query.Platform, "platform", "", "only this platform")
	cmd.Flags().StringVar(&query.AccountID, "account-id", "", "only this connected account")
	cmd.Flags().StringVar(&query.Profile, "profile", "", "only accounts in this profile (name or id)")
	cmd.Flags().StringVar(&query.Source, "source", "", "all (default) or postatron for posts made with Postatron only")
	return cmd
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
	fmt.Fprintf(w, "Created:   %s%s\n", p.CreatedAt.UTC().Format(time.RFC3339), byWhom(p.CreatedBy))
	if p.UpdatedBy != nil && p.UpdatedAt != nil {
		fmt.Fprintf(w, "Updated:   %s%s\n", p.UpdatedAt.UTC().Format(time.RFC3339), byWhom(p.UpdatedBy))
	}
	if p.Queue != nil {
		name := p.Queue.Name
		if name == "" {
			name = p.Queue.ID + " (deleted)"
		}
		fmt.Fprintf(w, "Queue:     %s\n", name)
	}
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

func byWhom(a *apiv1.PostAuthor) string {
	if a == nil || a.Name == "" {
		return ""
	}
	return " by " + a.Name
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
	if r.XLinkPosts.Limit > 0 {
		fmt.Fprintf(w, "X link posts:      %d / %d (%d remaining)\n", r.XLinkPosts.Used, r.XLinkPosts.Limit, r.XLinkPosts.Remaining)
	} else {
		// Counted for visibility, but no longer capped.
		fmt.Fprintf(w, "X link posts:      %d (no cap)\n", r.XLinkPosts.Used)
	}
	fmt.Fprintf(w, "Link posts (all):  %d\n", r.LinkPosts.Used)
	fmt.Fprintf(w, "Accounts:          %d / %d\n", r.Plan.ConnectedAccounts, r.Plan.MaxConnectedAccounts)
	writes := r.RateLimits.WritesPerMinute
	if writes == 0 {
		// A server older than writes_per_minute.
		writes = r.RateLimits.WritesPerHour / 60
	}
	fmt.Fprintf(w, "Rate limits:       %d writes, %d reads, %d uploads per minute\n", writes, r.RateLimits.ReadsPerMinute, r.RateLimits.UploadsPerMinute)

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

func printAnalytics(w io.Writer, r apiv1.AnalyticsReport) {
	fmt.Fprintf(w, "Range: %s (%s to %s)\n", r.Range.Key, dateOf(r.Range.Start), dateOf(r.Range.End))
	fmt.Fprintf(w, "Engagement rate: %.2f%% (%s)\n", r.Summary.EngagementRate, r.Summary.EngagementRateBasis)
	fmt.Fprintf(w, "Engagements: %d  Reach: %d  Followers: %d  Posts: %d\n",
		r.Summary.Engagements, r.Summary.Reach, r.Summary.Followers, r.Summary.PostsThisPeriod)

	if len(r.PerPlatform) > 0 {
		fmt.Fprintln(w, "\nPer platform:")
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "  PLATFORM\tPOSTS\tLIKES\tCOMMENTS\tSHARES\tVIEWS")
		for _, row := range r.PerPlatform {
			fmt.Fprintf(tw, "  %s\t%d\t%d\t%d\t%d\t%d\n", row.Platform, row.Posts, row.Likes, row.Comments, row.Shares, row.Views)
		}
		_ = tw.Flush()
	}
	if len(r.TopPosts) > 0 {
		fmt.Fprintln(w, "\nTop posts:")
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "  PLATFORM\tPUBLISHED\tENGAGEMENT\tTEXT")
		for _, p := range r.TopPosts {
			fmt.Fprintf(tw, "  %s\t%s\t%d\t%s\n", p.Platform, dateOf(p.PublishedAt), p.Engagement, truncate(oneLine(p.Text), 50))
		}
		_ = tw.Flush()
	}
	for _, note := range r.Notes {
		fmt.Fprintf(w, "\nNote: %s", note)
	}
	if len(r.Notes) > 0 {
		fmt.Fprintln(w)
	}
}

// dateOf trims an RFC 3339 timestamp to its date.
func dateOf(timestamp string) string {
	if len(timestamp) >= 10 {
		return timestamp[:10]
	}
	return timestamp
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
