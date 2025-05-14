package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/iwanbk/postgres-mcp-go/internal/server"
)

func main() {
	// Define the database URL flag
	databaseURL := flag.String("database_url", "", "Database URL (e.g., postgresql://username:password@localhost/mydb)")

	// Define SSE server flags
	sseAddr := flag.String("sse_addr", "", "Enable HTTP server with SSE support on the specified address (e.g., :8080)")

	// Parse the command-line flags
	flag.Parse()

	// Check if a database URL was provided
	if *databaseURL == "" {
		fmt.Fprintln(os.Stderr, "Please provide a database URL using the -database_url flag")
		fmt.Fprintln(os.Stderr, "Usage: postgres-mcp -database_url=<database-url>")
		fmt.Fprintln(os.Stderr, "Example: postgres-mcp -database_url=postgresql://username:password@localhost/mydb")
		os.Exit(1)
	}

	// Create a new PostgreSQL MCP server
	s, err := server.New(*databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	// Set up the server with resources and tools
	if err := s.Setup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set up server: %v\n", err)
		os.Exit(1)
	}

	// Start the server
	fmt.Fprintln(os.Stderr, "Starting PostgreSQL MCP server...")

	// Check if SSE server is enabled
	if *sseAddr != "" {
		// Start HTTP server with SSE in a goroutine
		go func() {
			fmt.Fprintf(os.Stderr, "Starting HTTP server with SSE support on %s\n", *sseAddr)
			if err := s.ServeHTTP(*sseAddr); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
				os.Exit(1)
			}
		}()
	}

	// Start the stdio-based MCP server
	if err := s.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
