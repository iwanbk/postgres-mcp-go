package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iwanbk/postgres-mcp-go/internal/server"
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
	if err := s.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
