CREATE TABLE IF NOT EXISTS keys (
    id           TEXT PRIMARY KEY,
    label        TEXT NOT NULL DEFAULT '',
    api_key_enc  TEXT NOT NULL,
    state        TEXT NOT NULL DEFAULT 'active' CHECK (state IN ('active','cooldown','disabled','revoked')),
    plan_type    TEXT NOT NULL DEFAULT 'unknown',
    cooldown_until TIMESTAMP,
    last_used_at   TIMESTAMP,
    last_error     TEXT NOT NULL DEFAULT '',
    request_count  INTEGER NOT NULL DEFAULT 0,
    success_count  INTEGER NOT NULL DEFAULT 0,
    fail_count     INTEGER NOT NULL DEFAULT 0,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS request_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key_id      TEXT NOT NULL,
    method      TEXT NOT NULL,
    path        TEXT NOT NULL,
    status_code INTEGER NOT NULL,
    latency_ms  INTEGER NOT NULL DEFAULT 0,
    error_msg   TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (key_id) REFERENCES keys(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_request_log_key ON request_log(key_id);
CREATE INDEX IF NOT EXISTS idx_request_log_created ON request_log(created_at);
