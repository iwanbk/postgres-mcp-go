package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

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
	// SSE related fields
	httpServer    *http.Server
	sseClients    map[string]chan string
	sseClientsMux sync.Mutex
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
		db:         db,
		server:     s,
		sseClients: make(map[string]chan string),
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

// ServeHTTP starts the MCP server with HTTP and SSE support
func (s *PostgresMCPServer) ServeHTTP(addr string) error {
	mux := http.NewServeMux()

	// SSE endpoint for real-time updates
	mux.HandleFunc("/sse", s.handleSSE)

	// MCP HTTP endpoint
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		// Handle MCP requests over HTTP
		if r.Method == http.MethodPost {
			var request map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				http.Error(w, fmt.Sprintf("Error decoding request: %v", err), http.StatusBadRequest)
				return
			}

			// Convert request to JSON
			requestJSON, err := json.Marshal(request)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error marshaling request: %v", err), http.StatusInternalServerError)
				return
			}

			// Process the MCP request
			response := s.server.HandleMessage(context.Background(), requestJSON)

			// Send the response
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)

			// Broadcast the response to SSE clients
			responseJSON, _ := json.Marshal(response)
			s.broadcastSSE(string(responseJSON))
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "PostgreSQL MCP Server is running")
	})

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	fmt.Printf("Starting HTTP server on %s\n", addr)
	return s.httpServer.ListenAndServe()
}

// handleSSE handles Server-Sent Events connections
func (s *PostgresMCPServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a unique client ID
	clientID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create a channel for this client
	messageChan := make(chan string)

	// Register the client
	s.sseClientsMux.Lock()
	s.sseClients[clientID] = messageChan
	s.sseClientsMux.Unlock()

	// Remove the client when the connection is closed
	defer func() {
		s.sseClientsMux.Lock()
		delete(s.sseClients, clientID)
		close(messageChan)
		s.sseClientsMux.Unlock()
	}()

	// Send initial connection message
	fmt.Fprintf(w, "data: %s\n\n", "{\"event\":\"connected\",\"clientId\":\""+clientID+"\"}")
	w.(http.Flusher).Flush()

	// Keep the connection open
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			// Client closed the connection
			return
		case msg := <-messageChan:
			// Send the message to the client
			fmt.Fprintf(w, "data: %s\n\n", msg)
			w.(http.Flusher).Flush()
		case <-time.After(30 * time.Second):
			// Send a keep-alive ping every 30 seconds
			fmt.Fprintf(w, ":\n\n") // Comment line as keep-alive
			w.(http.Flusher).Flush()
		}
	}
}

// broadcastSSE sends a message to all connected SSE clients
func (s *PostgresMCPServer) broadcastSSE(message string) {
	s.sseClientsMux.Lock()
	defer s.sseClientsMux.Unlock()

	for _, clientChan := range s.sseClients {
		// Non-blocking send
		select {
		case clientChan <- message:
		default:
			// Skip clients with full buffers
		}
	}
}

// Close closes the server and database connection
func (s *PostgresMCPServer) Close() error {
	// Close the HTTP server if it exists
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}

	// Close all SSE client channels
	s.sseClientsMux.Lock()
	for id, ch := range s.sseClients {
		close(ch)
		delete(s.sseClients, id)
	}
	s.sseClientsMux.Unlock()

	// Close the database connection
	return s.db.Close()
}
