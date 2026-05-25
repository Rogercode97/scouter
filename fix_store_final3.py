import re

with open("internal/store/store.go", "r") as f:
    text = f.read()

# Add Metrics to Symbol struct
text = text.replace(
    '	AISummary      string  `json:"ai_summary,omitempty"`\n}',
    '	AISummary      string  `json:"ai_summary,omitempty"`\n	Metrics        *types.SemanticMetrics `json:"metrics,omitempty"`\n}'
)

# Migration
metrics_check = """	// Dynamic column check for 'metrics_json'
	hasMetrics, err := hasColumn(ctx, tx, "symbols", "metrics_json")
	if err != nil {
		return fmt.Errorf("failed to check column symbols.metrics_json: %w", err)
	}
	if !hasMetrics {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE symbols ADD COLUMN metrics_json TEXT DEFAULT '{}';"); err != nil {
			return fmt.Errorf("failed to alter table symbols (metrics_json): %w", err)
		}
	}
"""
text = text.replace("	// Dynamic column check for 'ai_summary'", metrics_check + "\n	// Dynamic column check for 'ai_summary'")

# SaveSymbol INSERT
save_sql_old = '"INSERT INTO symbols (name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, pagerank, churn_score, ai_summary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"'
save_sql_new = '"INSERT INTO symbols (name, type, package_path, receiver_type, signature, doc, path, start_byte, end_byte, start_line, start_col, end_line, structural_hash, pagerank, churn_score, ai_summary, metrics_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"'
save_args_old = 'sym.PageRank, sym.ChurnScore, sym.AISummary)'
save_args_new = 'sym.PageRank, sym.ChurnScore, sym.AISummary, metricsJSON)'

json_block = """
	metricsJSON := "{}"
	if sym.Metrics != nil {
		if b, err := json.Marshal(sym.Metrics); err == nil {
			metricsJSON = string(b)
		}
	}
"""

text = text.replace(save_sql_old, save_sql_new)
text = text.replace(save_args_old, save_args_new)
text = text.replace('	_, err := s.exec(ctx, "INSERT INTO symbols', json_block + '	_, err := s.exec(ctx, "INSERT INTO symbols')

# SELECT SQL
text = re.sub(r'symbols\.ai_summary\s*\n\s*FROM', 'symbols.ai_summary, symbols.metrics_json\n            FROM', text)
text = re.sub(r'churn_score, ai_summary\s*\n\s*FROM', 'churn_score, ai_summary, metrics_json\n            FROM', text)
text = re.sub(r'ai_summary FROM symbols', 'ai_summary, metrics_json FROM symbols', text)

# rows.Scan
text = text.replace('&sym.AISummary)', '&sym.AISummary, &metricsJSON)')

# Insert var metricsJSON string
def inject_var(match):
    whitespace = match.group(1)
    line = match.group(0)
    if 'metricsJSON' in line:
        return f"{whitespace}var metricsJSON string\n{line}"
    return line

text = re.sub(r'([ \t]+)(if err := rows\.Scan\(&sym\.Name.*)', inject_var, text)

# Insert JSON Unmarshal
unmarshal = r"""\1if metricsJSON != "{}" && metricsJSON != "" {
\1	var m types.SemanticMetrics
\1	if err := json.Unmarshal([]byte(metricsJSON), &m); err == nil {
\1		sym.Metrics = &m
\1	}
\1}
\1results = append(results, sym)"""

text = re.sub(r'([ \t]+)results = append\(results, sym\)', unmarshal, text)

with open("internal/store/store.go", "w") as f:
    f.write(text)

