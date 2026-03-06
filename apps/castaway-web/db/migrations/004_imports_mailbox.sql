CREATE TABLE imports (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    instance_id BIGINT REFERENCES instances(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    season INTEGER NOT NULL,
    content_type TEXT NOT NULL,
    payload TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('accepted', 'validated', 'applied', 'failed')),
    error TEXT,
    submissions_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX imports_instance_id_idx ON imports(instance_id);
CREATE INDEX imports_status_idx ON imports(status);
