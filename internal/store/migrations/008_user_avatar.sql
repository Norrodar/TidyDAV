-- Add avatar_url for OIDC picture claim and accent for future use.
ALTER TABLE users ADD COLUMN avatar_url TEXT NOT NULL DEFAULT '';
