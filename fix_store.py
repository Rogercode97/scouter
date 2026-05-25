import re

with open("internal/store/store.go", "r") as f:
    content = f.read()

# Add dynamic column check for metrics_json
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

# Update SaveSymbol
content = content.replace(
    'pagerank, churn_score, ai_summary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", sym.Name, sym.Type, sym.PackagePath, sym.ReceiverType, sym.Signature, sym.Doc, sym.Path, sym.StartByte, sym.EndByte, sym.StartLine, sym.StartCol, sym.EndLine, sym.StructuralHash, sym.PageRank, sym.ChurnScore, sym.AISummary)',
    'pagerank, churn_score, ai_summary, metrics_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", sym.Name, sym.Type, sym.PackagePath, sym.ReceiverType, sym.Signature, sym.Doc, sym.Path, sym.StartByte, sym.EndByte, sym.StartLine, sym.StartCol, sym.EndLine, sym.StructuralHash, sym.PageRank, sym.ChurnScore, sym.AISummary, metricsJSON)'
)

# Insert JSON marshaling before SaveSymbol exec
json_marshal = """
	metricsJSON := "{}"
	if sym.Metrics != nil {
		if b, err := json.Marshal(sym.Metrics); err == nil {
			metricsJSON = string(b)
		}
	}
	_, err := s.exec(ctx, "INSERT"""

content = content.replace('_, err := s.exec(ctx, "INSERT', json_marshal)

# Update SELECTs
content = content.replace(', symbols.ai_summary', ', symbols.ai_summary, symbols.metrics_json')
content = content.replace(', churn_score, ai_summary', ', churn_score, ai_summary, metrics_json')

# Update rows.Scan
scan_replace = """
		var metricsJSON string
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Relevance, &sym.PageRank, &sym.ChurnScore, &sym.AISummary, &metricsJSON); err != nil {
"""
content = content.replace(
    '\t\tif err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Relevance, &sym.PageRank, &sym.ChurnScore, &sym.AISummary); err != nil {',
    scan_replace
)

scan_replace2 = """
			var metricsJSON string
			if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Relevance, &sym.PageRank, &sym.ChurnScore, &sym.AISummary, &metricsJSON); err != nil {
"""
content = content.replace(
    '\t\t\tif err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Relevance, &sym.PageRank, &sym.ChurnScore, &sym.AISummary); err != nil {',
    scan_replace2
)

scan_replace3 = """
		var metricsJSON string
		if err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Indegree, &sym.PageRank, &sym.ChurnScore, &sym.AISummary, &metricsJSON); err != nil {
"""
content = content.replace(
    '\t\tif err := rows.Scan(&sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.StartCol, &sym.EndLine, &sym.StructuralHash, &sym.Indegree, &sym.PageRank, &sym.ChurnScore, &sym.AISummary); err != nil {',
    scan_replace3
)

# And unmarshal JSON after scanning
# There are multiple append operations: results = append(results, sym)
unmarshal_code = """
		if metricsJSON != "{}" && metricsJSON != "" {
			var m SemanticMetrics
			if err := json.Unmarshal([]byte(metricsJSON), &m); err == nil {
				sym.Metrics = &m
			}
		}
		results = append"""
content = content.replace("\t\tresults = append", unmarshal_code)

unmarshal_code2 = """
			if metricsJSON != "{}" && metricsJSON != "" {
				var m SemanticMetrics
				if err := json.Unmarshal([]byte(metricsJSON), &m); err == nil {
					sym.Metrics = &m
				}
			}
			results = append"""
content = content.replace("\t\t\tresults = append", unmarshal_code2)

with open("internal/store/store.go", "w") as f:
    f.write(content)
