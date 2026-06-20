-- Password-reset tokens (the plaintext token is emailed; only its hash is stored).
CREATE TABLE password_resets (
    token_hash TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL
);

CREATE INDEX idx_password_resets_expires_at ON password_resets (expires_at);
