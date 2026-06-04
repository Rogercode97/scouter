CREATE TABLE IF NOT EXISTS file_index (
    path TEXT PRIMARY KEY,
    mtime INTEGER NOT NULL,
    hash TEXT NOT NULL,
    ast_json TEXT NOT NULL,
    project TEXT NOT NULL,
    freshness INTEGER DEFAULT 0 NOT NULL
);

CREATE TABLE IF NOT EXISTS directory_hashes (
    path TEXT PRIMARY KEY, 
    hash TEXT NOT NULL, 
    mtime INTEGER NOT NULL
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
    metrics_json TEXT DEFAULT '{}' NOT NULL,
    FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE
);

CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(name, type, signature, doc, path, content='symbols', content_rowid='id');

CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN 
    INSERT INTO symbols_fts(rowid, name, type, signature, doc, path) VALUES (new.id, new.name, new.type, new.signature, new.doc, new.path); 
END;

CREATE TRIGGER IF NOT EXISTS symbols_ad AFTER DELETE ON symbols BEGIN 
    INSERT INTO symbols_fts(symbols_fts, rowid, name, type, signature, doc, path) VALUES('delete', old.id, old.name, old.type, old.signature, old.doc, old.path); 
END;

CREATE TRIGGER IF NOT EXISTS symbols_au AFTER UPDATE ON symbols BEGIN 
    INSERT INTO symbols_fts(symbols_fts, rowid, name, type, signature, doc, path) VALUES('delete', old.id, old.name, old.type, old.signature, old.doc, old.path); 
    INSERT INTO symbols_fts(rowid, name, type, signature, doc, path) VALUES (new.id, new.name, new.type, new.signature, new.doc, new.path); 
END;

CREATE INDEX IF NOT EXISTS idx_symbols_path ON symbols(path);
CREATE INDEX IF NOT EXISTS idx_symbols_resolution ON symbols(name, path);

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

CREATE INDEX IF NOT EXISTS idx_calls_path ON calls(path);
CREATE INDEX IF NOT EXISTS idx_calls_callee ON calls(callee_name);
CREATE INDEX IF NOT EXISTS idx_calls_impact ON calls(callee_name, callee_path);
CREATE UNIQUE INDEX IF NOT EXISTS idx_calls_unique ON calls(caller_name, callee_name, path, line, link_type);

CREATE TABLE IF NOT EXISTS dependencies (
    id INTEGER PRIMARY KEY AUTOINCREMENT, 
    name TEXT NOT NULL, 
    version TEXT, 
    type TEXT, 
    project TEXT, 
    direct INTEGER
);

CREATE TABLE IF NOT EXISTS test_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT, 
    test_name TEXT NOT NULL, 
    status TEXT NOT NULL, 
    error_message TEXT, 
    stack_trace TEXT, 
    target_symbol TEXT, 
    duration_ms INTEGER, 
    project TEXT, 
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_test_results_symbol ON test_results(target_symbol);

CREATE TABLE IF NOT EXISTS violations (
    id INTEGER PRIMARY KEY AUTOINCREMENT, 
    rule_id TEXT, 
    file_path TEXT, 
    message TEXT, 
    severity TEXT, 
    start_line INTEGER, 
    start_col INTEGER, 
    text TEXT, 
    FOREIGN KEY(file_path) REFERENCES file_index(path) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_violations_file ON violations(file_path);

CREATE TABLE IF NOT EXISTS symbol_usage (
    id INTEGER PRIMARY KEY AUTOINCREMENT, 
    symbol_id INTEGER NOT NULL, 
    environment TEXT NOT NULL, 
    last_used INTEGER NOT NULL, 
    hit_count INTEGER NOT NULL, 
    UNIQUE(symbol_id, environment), 
    FOREIGN KEY(symbol_id) REFERENCES symbols(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS flows (
    id INTEGER PRIMARY KEY AUTOINCREMENT, 
    source TEXT NOT NULL, 
    sink TEXT NOT NULL, 
    type TEXT NOT NULL, 
    path TEXT NOT NULL, 
    line INTEGER NOT NULL, 
    FOREIGN KEY(path) REFERENCES file_index(path) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_flows_path ON flows(path);
CREATE INDEX IF NOT EXISTS idx_flows_sink ON flows(sink);

CREATE VIRTUAL TABLE IF NOT EXISTS vec_ast USING vec0(
    embedding float[384]
);

CREATE TABLE IF NOT EXISTS vec_ast_meta (
    rowid INTEGER PRIMARY KEY,
    symbol_id INTEGER UNIQUE,
    FOREIGN KEY(symbol_id) REFERENCES symbols(id) ON DELETE CASCADE
);
