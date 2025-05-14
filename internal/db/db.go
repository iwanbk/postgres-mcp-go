package db

import (
	"fmt"
	"net/url"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// DB represents a database connection
type DB struct {
	conn            *sqlx.DB
	resourceBaseURL string
}

// New creates a new DB instance
func New(databaseURL string) (*DB, error) {
	// Parse the database URL to create the resource base URL
	parsedURL, err := url.Parse(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Create resource base URL (postgres:// instead of postgresql://)
	resourceBaseURL := *parsedURL
	resourceBaseURL.Scheme = "postgres"
	// Remove password for security
	resourceBaseURL.User = url.User(parsedURL.User.Username())

	// Connect to the database
	conn, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &DB{
		conn:            conn,
		resourceBaseURL: resourceBaseURL.String(),
	}, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.conn.Close()
}

// ResourceBaseURL returns the base URL for resources
func (d *DB) ResourceBaseURL() string {
	return d.resourceBaseURL
}

// GetTableNames returns all table names in the public schema
func (d *DB) GetTableNames() ([]string, error) {
	var tableNames []string
	query := "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'"
	err := d.conn.Select(&tableNames, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get table names: %w", err)
	}
	return tableNames, nil
}

// TableColumn represents a column in a database table
type TableColumn struct {
	ColumnName string `db:"column_name"`
	DataType   string `db:"data_type"`
}

// GetTableSchema returns the schema for a specific table
func (d *DB) GetTableSchema(tableName string) ([]TableColumn, error) {
	var columns []TableColumn
	query := "SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1"
	err := d.conn.Select(&columns, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get table schema: %w", err)
	}
	return columns, nil
}

// ExecuteReadOnlyQuery executes a read-only SQL query
func (d *DB) ExecuteReadOnlyQuery(query string) ([]map[string]interface{}, error) {
	// Begin a read-only transaction
	tx, err := d.conn.Beginx()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Set transaction to read-only
	_, err = tx.Exec("SET TRANSACTION READ ONLY")
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to set transaction to read-only: %w", err)
	}

	// Execute the query
	rows, err := tx.Queryx(query)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Process the results
	result := []map[string]interface{}{}
	for rows.Next() {
		row := make(map[string]interface{})
		if err := rows.MapScan(row); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		result = append(result, row)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	// Rollback the transaction (since it's read-only, there's nothing to commit)
	if err := tx.Rollback(); err != nil {
		return nil, fmt.Errorf("failed to rollback transaction: %w", err)
	}

	return result, nil
}
