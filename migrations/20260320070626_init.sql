-- +goose Up
SELECT 'up SQL query';
CREATE TABLE IF NOT EXISTS example (
    id UUID PRIMARY KEY DEFAULT get_radom_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
SELECT 'down SQL query';
DROP TABLE IF EXISTS example;
