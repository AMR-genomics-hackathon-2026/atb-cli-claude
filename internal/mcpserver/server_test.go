package mcpserver

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolRegistration(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Name: "atb", Version: "1.0"}, nil)

	dataDir := t.TempDir()

	mcp.AddTool(s, &mcp.Tool{Name: "atb_query", Description: "Search bacterial genomes"}, makeQueryHandler(dataDir))
	mcp.AddTool(s, &mcp.Tool{Name: "atb_amr", Description: "Query AMR genes"}, makeAMRHandler(dataDir))
	mcp.AddTool(s, &mcp.Tool{Name: "atb_info", Description: "Get sample metadata"}, makeInfoHandler(dataDir))
	mcp.AddTool(s, &mcp.Tool{Name: "atb_stats", Description: "Database statistics"}, makeStatsHandler(dataDir))
	mcp.AddTool(s, &mcp.Tool{Name: "atb_species_list", Description: "List species"}, makeSpeciesListHandler(dataDir))

	// Connect via in-memory transport and verify 5 tools are listed.
	t1, t2 := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := s.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	wantTools := map[string]bool{
		"atb_query":        false,
		"atb_amr":          false,
		"atb_info":         false,
		"atb_stats":        false,
		"atb_species_list": false,
	}

	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("listing tools: %v", err)
		}
		if _, ok := wantTools[tool.Name]; ok {
			wantTools[tool.Name] = true
		}
	}

	for name, found := range wantTools {
		if !found {
			t.Errorf("tool %q not registered", name)
		}
	}
}
