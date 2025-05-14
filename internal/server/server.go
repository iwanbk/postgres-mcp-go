package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/iwanbk/postgres-mcp-go/internal/db"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// The schema path component for resource URIs
const schemaPath = "schema"

// PostgresMCPServer represents a PostgreSQL MCP server
type PostgresMCPServer struct {
	db     *db.DB
	server *server.MCPServer
}

// New creates a new PostgreSQL MCP server
func New(databaseURL string) (*PostgresMCPServer, error) {
	// Create the database connection
	db, err := db.New(databaseURL)
	if err != nil {
		return nil, err
	}

	// Create the MCP server
	s := server.NewMCPServer(
		"postgres-mcp-go", // Server name
		"0.1.0",           // Version
	)

	return &PostgresMCPServer{
		db:     db,
		server: s,
	}, nil
}

// Setup configures the MCP server with resources and tools
func (s *PostgresMCPServer) Setup() error {
	// Add resources for each table schema
	tableNames, err := s.db.GetTableNames()
	if err != nil {
		return fmt.Errorf("failed to get table names: %w", err)
	}

	for _, tableName := range tableNames {
		// Create a resource for each table schema
		resourceURI := fmt.Sprintf("%s/%s/%s", s.db.ResourceBaseURL(), tableName, schemaPath)
		resourceName := fmt.Sprintf("\"%s\" database schema", tableName)

		// Create the resource
		resource := mcp.NewResource(
			resourceURI,
			resourceName,
			mcp.WithResourceDescription(fmt.Sprintf("Schema information for table %s", tableName)),
			mcp.WithMIMEType("application/json"),
		)

		// Capture the tableName in a closure for the handler
		tableNameCopy := tableName

		// Add the resource with its handler
		s.server.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			// Get the schema for this table
			schema, err := s.db.GetTableSchema(tableNameCopy)
			if err != nil {
				return nil, fmt.Errorf("failed to get schema for table %s: %w", tableNameCopy, err)
			}

			// Convert the schema to JSON
			schemaJSON, err := json.MarshalIndent(schema, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal schema to JSON: %w", err)
			}

			// Return the schema as a resource content
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      request.Params.URI,
					MIMEType: "application/json",
					Text:     string(schemaJSON),
				},
			}, nil
		})
	}

	// Add the query tool
	queryTool := mcp.NewTool("query",
		mcp.WithDescription("Run a read-only SQL query"),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("The SQL query to execute"),
		),
	)

	// Add the tool with its handler
	s.server.AddTool(queryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract the SQL query from the request
		sql, ok := request.Params.Arguments["sql"].(string)
		if !ok {
			return mcp.NewToolResultError("SQL query is required"), nil
		}

		// Execute the query
		result, err := s.db.ExecuteReadOnlyQuery(sql)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("Failed to execute query", err), nil
		}

		// Convert the result to JSON
		resultJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultErrorFromErr("Failed to marshal result to JSON", err), nil
		}

		// Return the result
		return mcp.NewToolResultText(string(resultJSON)), nil
	})

	return nil
}

// Serve starts the MCP server using stdio
func (s *PostgresMCPServer) Serve() error {
	return server.ServeStdio(s.server)
}

// Close closes the server and database connection
func (s *PostgresMCPServer) Close() error {
	return s.db.Close()
}
