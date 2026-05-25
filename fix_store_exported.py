import re

with open("internal/store/store.go", "r") as f:
    text = f.read()

# Migration check for 'exported'
exported_check = """	// Dynamic column check for 'exported'
	hasExported, err := hasColumn(ctx, tx, "symbols", "exported")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.exported: %w", err)
	}
	if !hasExported {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN exported INTEGER DEFAULT 0;"); err != nil {
			return fmt.Errorf("failed to alter table symbols (exported): %w", err)
		}
	}
"""
text = text.replace("	// Dynamic column check for 'metrics_json'", exported_check + "\n	// Dynamic column check for 'metrics_json'")

# SaveSymbol INSERT
# It currently has: ai_summary, metrics_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
# and sym.AISummary, metricsJSON)
text = text.replace(
    "ai_summary, metrics_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)\"",
    "ai_summary, metrics_json, exported) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)\""
)
text = text.replace(
    "sym.AISummary, metricsJSON)",
    "sym.AISummary, metricsJSON, sym.Exported)"
)

# SELECT SQL updates
# symbols.ai_summary, symbols.metrics_json
text = re.sub(r'symbols\.ai_summary, symbols\.metrics_json\s*\n\s*FROM', 'symbols.ai_summary, symbols.metrics_json, symbols.exported\n            FROM', text)
text = re.sub(r'ai_summary, metrics_json\s*\n\s*FROM', 'ai_summary, metrics_json, exported\n            FROM', text)
text = re.sub(r'ai_summary, metrics_json FROM symbols', 'ai_summary, metrics_json, exported FROM symbols', text)

# rows.Scan
# &sym.AISummary, &metricsJSON)
text = text.replace('&sym.AISummary, &metricsJSON)', '&sym.AISummary, &metricsJSON, &sym.Exported)')

with open("internal/store/store.go", "w") as f:
    f.write(text)
