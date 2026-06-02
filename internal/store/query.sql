-- name: SaveFileIndex :exec
INSERT INTO file_index (
    path, mtime, hash, ast_json, project, freshness
) VALUES (
    ?, ?, ?, ?, ?, ?
)
ON CONFLICT(path) DO UPDATE SET
    mtime = excluded.mtime,
    hash = excluded.hash,
    ast_json = excluded.ast_json,
    project = excluded.project,
    freshness = excluded.freshness;

-- name: ClearSymbols :exec
DELETE FROM symbols WHERE path = ?;

-- name: SaveSymbol :exec
INSERT INTO symbols (
    name, type, package_path, receiver_type, signature, doc,
    path, start_byte, end_byte, start_line, start_col, end_line,
    structural_hash, indegree, relevance, pagerank, churn_score,
    runtime_hits, ai_summary
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: SaveCall :exec
INSERT INTO calls (
    caller_name, callee_name, path, line, callee_path, link_type, indegree, body
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: SearchSymbols :many
SELECT * FROM symbols
WHERE name LIKE '%' || ? || '%' OR signature LIKE '%' || ? || '%'
LIMIT ? OFFSET ?;

-- name: GetCallers :many
SELECT * FROM calls
WHERE callee_name = ?
LIMIT ? OFFSET ?;

-- name: GetCallees :many
SELECT * FROM calls
WHERE caller_name = ?;
