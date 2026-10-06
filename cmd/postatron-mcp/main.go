// Command postatron-mcp is the Postatron MCP server. It speaks MCP over stdio
// and forwards its tools to the public API with your API key.
//
//	POSTATRON_API_KEY=ptn_... postatron-mcp
//
// Optional: POSTATRON_API_URL overrides the API host (defaults to
// https://api.postatron.com).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/henrwal/postatron-cli/apiv1"
	"github.com/henrwal/postatron-cli/mcpserver"
)

func main() {
	apiKey := strings.TrimSpace(os.Getenv("POSTATRON_API_KEY"))
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "postatron-mcp: POSTATRON_API_KEY is not set. Create a key at https://postatron.com/dashboard/api")
		os.Exit(2)
	}

	client := apiv1.NewClient(os.Getenv("POSTATRON_API_URL"), apiKey)
	client.UserAgent = "postatron-mcp/" + mcpserver.Version

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := mcpserver.Run(ctx, client); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "postatron-mcp: %v\n", err)
		os.Exit(1)
	}
}
