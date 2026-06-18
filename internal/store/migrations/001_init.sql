-- Initial schema: users and sessions.

CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    kind          TEXT NOT NULL CHECK (kind IN ('password', 'oidc', 'secret')),
    email         TEXT,
    password_hash TEXT,
    oidc_subject  TEXT,
    secret_hash   TEXT,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_users_email ON users (email) WHERE email IS NOT NULL;
CREATE UNIQUE INDEX idx_users_oidc_subject ON users (oidc_subject) WHERE oidc_subject IS NOT NULL;
CREATE UNIQUE INDEX idx_users_secret_hash ON users (secret_hash) WHERE secret_hash IS NOT NULL;

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE INDEX idx_sessions_user_id ON sessions (user_id);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);
