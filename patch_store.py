import os
import re

path = "internal/store/store.go"

with open(path, "r") as f:
    content = f.read()

# 1. Replace InsertSemanticVector
insert_semantic_old = """func (s *storeImpl) InsertSemanticVector(ctx context.Context, symbolID int64, embedding []float32) error {
	embJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding: %w", err)
	}

	// Insert into vec_ast_meta first to get rowid
	res, err := s.exec(ctx, "INSERT INTO vec_ast_meta (symbol_id) VALUES (?) ON CONFLICT(symbol_id) DO NOTHING", symbolID)
	if err != nil {
		return fmt.Errorf("failed to insert vec_ast_meta: %w", err)
	}

	var rowID int64
	// If ON CONFLICT DO NOTHING triggered, res.LastInsertId() might be 0.
	// We should probably just select it if it exists.
	rowID, err = res.LastInsertId()
	if err != nil || rowID == 0 {
		err = s.queryRow(ctx, "SELECT rowid FROM vec_ast_meta WHERE symbol_id = ?", symbolID).Scan(&rowID)
		if err != nil {
			return fmt.Errorf("failed to get rowid for symbol %d: %w", symbolID, err)
		}
	}

	// Now insert/update vec_ast
	_, err = s.exec(ctx, "INSERT OR REPLACE INTO vec_ast (rowid, embedding) VALUES (?, ?)", rowID, string(embJSON))
	if err != nil {
		return fmt.Errorf("failed to insert vec_ast: %w", err)
	}
	return nil
}"""

insert_semantic_new = """func (s *storeImpl) InsertSemanticVector(ctx context.Context, symbolID int64, embedding []float32) error {
	vecData, err := ncruces.SerializeFloat32(embedding)
	if err != nil {
		return fmt.Errorf("failed to serialize embedding: %w", err)
	}

	_, err = s.exec(ctx, "INSERT OR REPLACE INTO vec_symbols (symbol_id, embedding) VALUES (?, ?)", symbolID, vecData)
	if err != nil {
		return fmt.Errorf("failed to insert vec_symbols: %w", err)
	}
	return nil
}"""

content = content.replace(insert_semantic_old, insert_semantic_new)

# 2. Add SearchHybrid
search_hybrid = """
const (
	DefaultNearestNeighborLimit = 100
	RrfKConstant                = 60
)

func (s *storeImpl) SearchHybrid(ctx context.Context, textQuery string, queryEmbedding []float32, limit int) ([]Symbol, error) {
	vecData, err := ncruces.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize embedding: %w", err)
	}

	query := fmt.Sprintf(`
WITH fts_matches AS (
    SELECT rowid as symbol_id, rank, row_number() OVER (ORDER BY rank) as rn
    FROM symbols_fts
    WHERE symbols_fts MATCH ?
    LIMIT %d
),
vec_matches AS (
    SELECT symbol_id, distance, row_number() OVER (ORDER BY distance) as rn
    FROM vec_symbols
    WHERE embedding MATCH ? AND k = %d
),
combined AS (
    SELECT
        COALESCE(f.symbol_id, v.symbol_id) as symbol_id,
        COALESCE(1.0 / (%d + f.rn), 0) + COALESCE(1.0 / (%d + v.rn), 0) as rrf_score
    FROM fts_matches f
    FULL OUTER JOIN vec_matches v ON f.symbol_id = v.symbol_id
)
SELECT s.id, s.name, s.type, s.package_path, s.receiver_type, s.signature, s.doc, s.path, s.start_byte, s.end_byte, s.start_line, s.end_line
FROM combined c
JOIN symbols s ON s.id = c.symbol_id
ORDER BY c.rrf_score DESC
LIMIT ?
`, DefaultNearestNeighborLimit, DefaultNearestNeighborLimit, RrfKConstant, RrfKConstant)

	rows, err := s.query(ctx, query, textQuery, vecData, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute hybrid search query: %w", err)
	}
	defer rows.Close()

	var res []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(&sym.ID, &sym.Name, &sym.Type, &sym.PackagePath, &sym.ReceiverType, &sym.Signature, &sym.Doc, &sym.Path, &sym.StartByte, &sym.EndByte, &sym.StartLine, &sym.EndLine); err != nil {
			return nil, fmt.Errorf("scan hybrid symbol failed: %w", err)
		}
		res = append(res, sym)
	}
	return res, nil
}
"""

content = content + search_hybrid

# 3. Add ncruces package to imports if not there (it is, but let's make sure it's accessible). It is imported as _ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
# We need to change the import from `_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"` to `"github.com/asg017/sqlite-vec-go-bindings/ncruces"`
content = content.replace('_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"', '"github.com/asg017/sqlite-vec-go-bindings/ncruces"')

with open(path, "w") as f:
    f.write(content)

