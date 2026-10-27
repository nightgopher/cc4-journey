BEGIN;

ALTER TABLE attendance_records ADD COLUMN IF NOT EXISTS source text NOT NULL DEFAULT 'manual';

-- The frontend and permissions already define finance; include it in account roles.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin','hr','manager','workplace_manager','employee','finance'));

CREATE TABLE IF NOT EXISTS agent_tool_audit (
    id serial PRIMARY KEY,
    actor text,
    action text,
    detail text,
    created_at timestamptz DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS order_edit_history (
    id serial PRIMARY KEY,
    order_detail_id integer NOT NULL REFERENCES daily_performance_order_details(id) ON DELETE CASCADE,
    edited_by integer REFERENCES employees(id),
    changes jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMIT;
