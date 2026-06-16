package astgrep

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/filter"
)

type Searcher struct{}

func NewSearcher() *Searcher {
	return &Searcher{}
}

func (s *Searcher) Search(ctx context.Context, pattern, path, ext string, limit, offset int) ([]engine.StructuralMatch, int, error) {
	action, ok := filter.GetAction("ast_grep")
	if !ok {
		return nil, 0, fmt.Errorf("ast_grep action not found in registry")
	}

	params := map[string]any{
		"pattern": pattern,
		"path":    path,
	}

	lang := ""
	switch ext {
	case ".go", "go":
		lang = "go"
	case ".ts", "ts":
		lang = "typescript"
	case ".js", "js":
		lang = "javascript"
	case ".rs", "rs":
		lang = "rust"
	case ".py", "py":
		lang = "python"
	}
	if lang != "" {
		params["lang"] = lang
	}

	input := filter.ActionResult{Metadata: map[string]any{"path": path}}
	res, err := action(ctx, input, params)
	if err != nil {
		return nil, 0, fmt.Errorf("Structural search failed: %v", err)
	}

	var results []engine.StructuralMatch
	for _, line := range res.Lines {
		var match filter.AstGrepMatch
		if err := json.Unmarshal([]byte(line), &match); err == nil {
			matchPath := match.File
			if matchPath == "" {
				matchPath = path
			}
			results = append(results, engine.StructuralMatch{
				Path:      matchPath,
				StartLine: match.Range.Start.Line,
				EndLine:   match.Range.End.Line,
				Content:   match.Text,
			})
		}
	}

	total := len(results)

	if offset >= total {
		results = []engine.StructuralMatch{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		results = results[offset:end]
	}

	return results, total, nil
}
