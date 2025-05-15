package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/iwanbk/postgres-mcp-go/internal/db"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Define the database URL flag
	databaseURL := flag.String("database_url", "", "Database URL (e.g., postgresql://username:password@localhost/mydb)")

	// Parse the command-line flags
	flag.Parse()

	// Check if a database URL was provided
	if *databaseURL == "" {
		fmt.Fprintln(os.Stderr, "Please provide a database URL using the -database_url flag")
		fmt.Fprintln(os.Stderr, "Usage: postgres-mcp -database_url=<database-url>")
		fmt.Fprintln(os.Stderr, "Example: postgres-mcp -database_url=postgresql://username:password@localhost/mydb")
		os.Exit(1)
	}

	db, err := db.New(*databaseURL)
	if err != nil {
		log.Fatalf("Failed to create database connection: %v", err)
	}
	defer db.Close()

	listTablesTool := mcp.NewTool(
		"list_table",
		mcp.WithDescription("list_tables"),
	)

	s := server.NewMCPServer(
		"go-mcp-postgres",
		"0.2.1",
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithLogging(),
	)

	s.AddTool(listTablesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		log.Printf("list tables tool called with request: %v", request)
		result, err := db.GetTableNames()
		if err != nil {
			return nil, nil
		}

		// Convert the schema to JSON
		schemaJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal schema to JSON: %w", err)
		}

		return mcp.NewToolResultText(string(schemaJSON)), nil
	})

	sseServer := server.NewSSEServer(s,
		server.WithBaseURL("http://127.0.0.1:8000"),
	)
	sseServer.Start(":8000")
}
