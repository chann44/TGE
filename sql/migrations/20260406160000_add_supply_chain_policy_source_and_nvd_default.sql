-- +goose Up
ALTER TABLE policy_sources
    ADD COLUMN IF NOT EXISTS supply_chain_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE policy_sources
    ALTER COLUMN nvd_enabled SET DEFAULT TRUE;

UPDATE policy_sources
SET nvd_enabled = TRUE
WHERE nvd_enabled = FALSE;

-- +goose Down
ALTER TABLE policy_sources
    DROP COLUMN IF EXISTS supply_chain_enabled;

ALTER TABLE policy_sources
    ALTER COLUMN nvd_enabled SET DEFAULT FALSE;
