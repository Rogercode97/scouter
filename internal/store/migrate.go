package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// The schema in schema.sql is written with CREATE TABLE IF NOT EXISTS, which is
// a no-op on a table that already exists. That makes it a complete description
// of a fresh database and no description at all of an old one: a database
// created by an earlier release keeps its old columns, and every query that
// names a new column then fails with "no such column".
//
// convergence closes that gap. The column set is parsed out of the embedded
// schema itself, so the ladder that converges an old database cannot drift from
// the schema it converges to: adding a column to schema.sql puts it on the
// ladder automatically.
type columnDef struct {
	name string
	ddl  string
}

var createTableRe = regexp.MustCompile(
	`(?is)^CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

var columnConstraintRe = regexp.MustCompile(
	`(?is)^(?:CONSTRAINT|PRIMARY\s+KEY|FOREIGN\s+KEY|UNIQUE|CHECK)\b`)

var identifierRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var (
	schemaColumns = parseSchemaColumns(schemaSQL)
	schemaOrder   = tableOrder(schemaSQL)
	schemaTables  = tableSet(schemaOrder)
)

func tableSet(order []string) map[string]bool {
	set := make(map[string]bool, len(order))
	for _, table := range order {
		set[table] = true
	}
	return set
}

// parseSchemaColumns extracts, per table, the column definitions declared in
// the schema. Virtual tables (fts5, vec0) are skipped: their columns are owned
// by the extension and ALTER TABLE does not apply to them.
func parseSchemaColumns(schema string) map[string][]columnDef {
	columns := map[string][]columnDef{}
	for _, table := range tableOrder(schema) {
		columns[table] = nil
	}
	for _, stmt := range strings.Split(schema, "\n\n") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		match := createTableRe.FindStringSubmatch(stmt)
		if match == nil {
			continue
		}
		body, ok := balancedBody(stmt)
		if !ok {
			continue
		}

		var defs []columnDef
		for _, part := range splitTopLevel(body) {
			part = strings.TrimSpace(part)
			if part == "" || columnConstraintRe.MatchString(part) {
				continue
			}
			fields := strings.Fields(part)
			if len(fields) < 2 {
				continue
			}
			name := strings.Trim(fields[0], "\"`[]")
			if !identifierRe.MatchString(name) {
				continue
			}
			defs = append(defs, columnDef{
				name: name,
				ddl:  strings.Join(fields[1:], " "),
			})
		}
		if len(defs) > 0 {
			columns[match[1]] = defs
		}
	}
	return columns
}

// tableOrder preserves the declaration order of schema.sql, so the ladder emits
// its statements in the same order on every run.
func tableOrder(schema string) []string {
	var order []string
	for _, stmt := range strings.Split(schema, "\n\n") {
		stmt = strings.TrimSpace(stmt)
		if match := createTableRe.FindStringSubmatch(stmt); match != nil {
			order = append(order, match[1])
		}
	}
	return order
}

// balancedBody returns the text between the outermost parentheses of a CREATE
// TABLE statement, honouring nesting and quoted literals so a comma inside
// either is not a separator.
func balancedBody(s string) (string, bool) {
	start := strings.Index(s, "(")
	if start < 0 {
		return "", false
	}
	depth := 0
	quote := byte(0)
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return s[start+1 : i], true
			}
		}
	}
	return "", false
}

func splitTopLevel(body string) []string {
	var parts []string
	depth := 0
	quote := byte(0)
	start := 0
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(' || c == '[':
			depth++
		case c == ')' || c == ']':
			depth--
		case c == ',' && depth == 0:
			parts = append(parts, body[start:i])
			start = i + 1
		}
	}
	parts = append(parts, body[start:])
	return parts
}

func migrate(ctx context.Context, tx *sql.Tx) error {
	queries := strings.Split(schemaSQL, "\n\n")
	for _, q := range queries {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

// convergeSchema brings an existing database up to the current schema by adding
// the columns the schema declares and the database lacks. It is idempotent: a
// converged database produces no statement at all.
func convergeSchema(ctx context.Context, tx *sql.Tx) error {
	for _, table := range schemaOrder {
		defs := schemaColumns[table]
		if len(defs) == 0 {
			continue
		}
		existing, err := tableColumns(ctx, tx, table)
		if err != nil {
			return fmt.Errorf("reading columns of %s: %w", table, err)
		}
		for _, def := range defs {
			if existing[def.name] {
				continue
			}
			if def.ddl == "" {
				return fmt.Errorf("adding %s.%s: no definition in schema", table, def.name)
			}
			// #nosec G201 -- table is validated by tableColumns against the
			// closed set parsed from the embedded schema, the column name came
			// from that same schema, and the definition text is emitted verbatim
			// from it.
			stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, def.name, def.ddl)
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("adding %s.%s: %w", table, def.name, err)
			}
		}
	}
	return nil
}

// tableColumns reads every column of one table in a single query. Resolving one
// column at a time costs a query per column, and this runs on every open: the
// CLI pays it on each invocation, so the whole table is read once and diffed in
// memory instead.
func tableColumns(ctx context.Context, tx *sql.Tx, table string) (map[string]bool, error) {
	if !schemaTables[table] {
		return nil, fmt.Errorf("unknown table: %s", table)
	}

	// #nosec G201 -- table is validated against schemaTables immediately
	// above, and that set is parsed from the embedded schema.sql at init, so it
	// is a closed set of identifiers with no caller-supplied text in it. The
	// column is never interpolated here: SQLite resolves it from the returned
	// rows and the ladder diffs the set in memory.
	query := fmt.Sprintf("SELECT name FROM pragma_table_info('%s')", table)
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying pragma_table_info for %s: %w", table, err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning column of %s: %w", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading columns of %s: %w", table, err)
	}
	return columns, nil
}

func hasColumn(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	if !identifierRe.MatchString(column) {
		return false, fmt.Errorf("invalid column name: %s", column)
	}
	columns, err := tableColumns(ctx, tx, table)
	if err != nil {
		return false, err
	}
	return columns[column], nil
}
