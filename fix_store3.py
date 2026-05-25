import re

with open("internal/store/store.go", "r") as f:
    content = f.read()

metrics_check = """
	// Dynamic column check for 'metrics_json'
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
content = re.sub(
    r'(\s*hasSummary, err := hasColumn\(ctx, tx, "symbols", "ai_summary"\))',
    metrics_check + r'\1',
    content
)

content = content.replace(
    'pagerank, churn_score, ai_summary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", sym.Name, sym.Type, sym.PackagePath, sym.ReceiverType, sym.Signature, sym.Doc, sym.Path, sym.StartByte, sym.EndByte, sym.StartLine, sym.StartCol, sym.EndLine, sym.StructuralHash, sym.PageRank, sym.ChurnScore, sym.AISummary)',
    'pagerank, churn_score, ai_summary, metrics_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", sym.Name, sym.Type, sym.PackagePath, sym.ReceiverType, sym.Signature, sym.Doc, sym.Path, sym.StartByte, sym.EndByte, sym.StartLine, sym.StartCol, sym.EndLine, sym.StructuralHash, sym.PageRank, sym.ChurnScore, sym.AISummary, metricsJSON)'
)

json_marshal = """
	metricsJSON := "{}"
	if sym.Metrics != nil {
		if b, err := json.Marshal(sym.Metrics); err == nil {
			metricsJSON = string(b)
		}
	}
	_, err := s.exec(ctx, "INSERT"""
content = content.replace('_, err := s.exec(ctx, "INSERT', json_marshal)

# Update SELECTs for sym queries
# Note that there are queries that select columns. We can regex match `symbols.ai_summary` and `churn_score, ai_summary` safely if they belong to symbols.
content = re.sub(r'symbols\.ai_summary([\s\n]*FROM)', r'symbols.ai_summary, symbols.metrics_json\1', content)
content = re.sub(r'churn_score, ai_summary([\s\n]*FROM)', r'churn_score, ai_summary, metrics_json\1', content)

# Now find all rows.Scan that map to sym.
# We have a few formats.
# format 1: &sym.Relevance, &sym.PageRank, &sym.ChurnScore, &sym.AISummary
# format 2: &sym.Indegree, &sym.PageRank, &sym.ChurnScore, &sym.AISummary
# We can replace the closing parentheses of these rows.Scan calls and add the metricsJSON variable.

content = re.sub(
    r'(rows\.Scan\([^)]*&sym\.AISummary)\)',
    r'\1, &metricsJSON)',
    content
)

# And then we need to insert the unmarshal before `results = append(results, sym)`
unmarshal_code = """
		if metricsJSON != "{}" && metricsJSON != "" {
			var m SemanticMetrics
			if err := json.Unmarshal([]byte(metricsJSON), &m); err == nil {
				sym.Metrics = &m
			}
		}
		results = append(results, sym)"""

# In the places where `metricsJSON` is now scanned, we need to declare it before the `Scan`.
# So let's find `if err := rows.Scan(...` where `&sym.AISummary` is present, and add `var metricsJSON string` before it.
def repl_scan(match):
    whitespace = match.group(1)
    scan_call = match.group(2)
    return f"{whitespace}var metricsJSON string\n{whitespace}{scan_call}"

content = re.sub(r'([ \t]*)(if err := rows\.Scan\([^)]*&sym\.AISummary, &metricsJSON\))', repl_scan, content)
content = re.sub(r'([ \t]*)(rows\.Scan\([^)]*&sym\.AISummary, &metricsJSON\))', repl_scan, content)

# Replace `results = append(results, sym)`
content = re.sub(r'[ \t]*results = append\(results, sym\)', unmarshal_code, content)

with open("internal/store/store.go", "w") as f:
    f.write(content)

