CREATE TABLE IF NOT EXISTS file_index (
    path TEXT PRIMARY KEY,
    mtime INTEGER NOT NULL,
    hash TEXT NOT NULL,
    ast_json TEXT NOT NULL,
    project TEXT NOT NULL,
    freshness INTEGER DEFAULT 0 NOT NULL
);

CREATE TABLE IF NOT EXISTS symbols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    package_path TEXT DEFAULT '' NOT NULL,
    receiver_type TEXT DEFAULT '' NOT NULL,
    signature TEXT DEFAULT '' NOT NULL,
    doc TEXT NOT NULL,
    path TEXT NOT NULL,
    start_byte INTEGER NOT NULL,
    end_byte INTEGER NOT NULL,
    start_line INTEGER NOT NULL,
    start_col INTEGER NOT NULL,
    end_line INTEGER NOT NULL,
    structural_hash TEXT DEFAULT '' NOT NULL,
    indegree INTEGER DEFAULT 0 NOT NULL,
    relevance REAL DEFAULT 0.0 NOT NULL,
    pagerank REAL DEFAULT 0.0 NOT NULL,
    churn_score REAL DEFAULT 0.0 NOT NULL,
    runtime_hits INTEGER DEFAULT 0 NOT NULL,
    ai_summary TEXT DEFAULT '' NOT NULL,
    FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS calls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    caller_name TEXT NOT NULL,
    callee_name TEXT NOT NULL,
    path TEXT NOT NULL,
    line INTEGER NOT NULL,
    callee_path TEXT DEFAULT '' NOT NULL,
    link_type TEXT DEFAULT 'call' NOT NULL,
    indegree INTEGER DEFAULT 0 NOT NULL,
    body TEXT DEFAULT '' NOT NULL,
    FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE
);
