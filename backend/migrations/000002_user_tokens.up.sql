CREATE TABLE user_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    token_type TEXT NOT NULL CHECK (token_type IN ('refresh', 'oauth', '2fa')),
    token_hash TEXT NOT NULL,
    family_id UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    replaced_by BIGINT REFERENCES user_tokens(id) ON DELETE RESTRICT,
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX user_tokens_hash_unique ON user_tokens (token_hash);
CREATE INDEX user_tokens_user_expires_idx ON user_tokens (user_id, expires_at);
