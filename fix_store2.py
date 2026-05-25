import re

with open("internal/store/store.go", "r") as f:
    content = f.read()

# I need to revert the incorrect unmarshal_code
bad_unmarshal = """
		if metricsJSON != "{}" && metricsJSON != "" {
			var m SemanticMetrics
			if err := json.Unmarshal([]byte(metricsJSON), &m); err == nil {
				sym.Metrics = &m
			}
		}
		results = append"""

content = content.replace(bad_unmarshal + "(results, c)", "\t\tresults = append(results, c)")
content = content.replace(bad_unmarshal + "(results, d)", "\t\tresults = append(results, d)")
content = content.replace(bad_unmarshal + "(results, r)", "\t\tresults = append(results, r)")
content = content.replace(bad_unmarshal + "(results, v)", "\t\tresults = append(results, v)")

bad_unmarshal2 = """
			if metricsJSON != "{}" && metricsJSON != "" {
				var m SemanticMetrics
				if err := json.Unmarshal([]byte(metricsJSON), &m); err == nil {
					sym.Metrics = &m
				}
			}
			results = append"""

content = content.replace(bad_unmarshal2 + "(results, c)", "\t\t\tresults = append(results, c)")
content = content.replace(bad_unmarshal2 + "(results, d)", "\t\t\tresults = append(results, d)")
content = content.replace(bad_unmarshal2 + "(results, r)", "\t\t\tresults = append(results, r)")
content = content.replace(bad_unmarshal2 + "(results, v)", "\t\t\tresults = append(results, v)")
content = content.replace(bad_unmarshal2 + "(results, p)", "\t\t\tresults = append(results, p)")

with open("internal/store/store.go", "w") as f:
    f.write(content)
