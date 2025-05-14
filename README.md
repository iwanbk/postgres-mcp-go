# PostgreSQL MCP Server in Go

A Go implementation of a Model Context Protocol (MCP) server for PostgreSQL. This server enables LLMs to inspect database schemas and execute read-only queries against PostgreSQL databases.

## Features

- Exposes PostgreSQL table schemas as resources
- Provides a tool to execute read-only SQL queries
- Uses sqlx for PostgreSQL connectivity
- Built with the mcp-go library

## Installation

```bash
go install github.com/iwanbk/postgres-mcp-go/cmd/postgres-mcp@latest
```

Or build from source:

```bash
git clone https://github.com/iwanbk/postgres-mcp-go.git
cd postgres-mcp-go
go build -o postgres-mcp ./cmd/postgres-mcp
```

## Usage

Run the server by providing a PostgreSQL connection URL:

```bash
postgres-mcp postgresql://username:password@localhost/mydb
```

### Resources

The server provides schema information for each table in the database:

- `postgres://<host>/<table>/schema` - JSON schema information for each table
  - Includes column names and data types
  - Automatically discovered from database metadata

### Tools

- `query` - Execute read-only SQL queries against the connected database
  - Input: `sql` (string): The SQL query to execute
  - All queries are executed within a READ ONLY transaction

## Security

This server only allows read-only operations. All queries are executed within a READ ONLY transaction to prevent any data modification.

## License

MIT
