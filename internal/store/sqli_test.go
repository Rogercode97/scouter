package store

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLInjection_hasColumn(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE symbols (name TEXT)")
	if err != nil {
		t.Fatal(err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	ctx := context.Background()

	// Normal case: column doesn't exist
	has, err := hasColumn(ctx, tx, "symbols", "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("Expected false for nonexistent column")
	}

	// Injection case: bypass check
	// table = symbols') --
	// query = SELECT 1 FROM pragma_table_info('symbols') --') WHERE name = ?
	// This will comment out the WHERE clause and return all columns from symbols, 
	// thus Scan will succeed if symbols has at least one column.
	has, err = hasColumn(ctx, tx, "symbols') --", "nonexistent")
	if err != nil {
		t.Fatalf("Injection failed with error: %v", err)
	}
	if has {
		t.Log("SUCCESS: SQL Injection proven! hasColumn returned true for nonexistent column due to injection.")
	} else {
		t.Error("FAILED: SQL Injection not proven (returned false)")
	}
}
