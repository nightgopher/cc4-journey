BEGIN;

CREATE TABLE mcp_credentials (
    id bigserial PRIMARY KEY,
    user_id integer NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issued_by integer NOT NULL REFERENCES users(id),
    label varchar(80) NOT NULL DEFAULT 'Agent',
    token_hash text NOT NULL UNIQUE CHECK (length(token_hash) = 64),
    token_prefix varchar(24) NOT NULL,
    access_mode text NOT NULL CHECK (access_mode IN ('read_only', 'read_write')),
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at timestamptz
);

CREATE UNIQUE INDEX mcp_credentials_one_active_user ON mcp_credentials(user_id) WHERE revoked_at IS NULL;

COMMIT;
