import re

with open("internal/store/store.go", "r") as f:
    text = f.read()

# 1. Migration
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

# 2. SaveSymbol INSERT
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

# 3. SELECT SQL
# There are two variations of ai_summary in SELECT queries that span across lines or end with it.
text = re.sub(r'symbols\.ai_summary\s*\n\s*FROM', 'symbols.ai_summary, symbols.metrics_json\n            FROM', text)
text = re.sub(r'churn_score, ai_summary\s*\n\s*FROM', 'churn_score, ai_summary, metrics_json\n            FROM', text)
text = re.sub(r'ai_summary FROM symbols', 'ai_summary, metrics_json FROM symbols', text)

# 4. rows.Scan
text = text.replace('&sym.AISummary)', '&sym.AISummary, &metricsJSON)')

# 5. Insert var metricsJSON string
# Match the exact lines calling rows.Scan with &sym.Name
text = re.sub(r'([ \t]+)(if err := rows\.Scan\(&sym\.Name)', r'\1var metricsJSON string\n\1\2', text)

# 6. Insert JSON Unmarshal
# Replace `results = append(results, sym)` preserving indentation
text = re.sub(r'([ \t]+)results = append\(results, sym\)', r'\1if metricsJSON != "{}" && metricsJSON != "" {\n\1	var m SemanticMetrics\n\1	if err := json.Unmarshal([]byte(metricsJSON), &m); err == nil {\n\1		sym.Metrics = &m\n\1	}\n\1}\n\1results = append(results, sym)', text)

with open("internal/store/store.go", "w") as f:
    f.write(text)

