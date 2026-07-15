-- Persist last successful deployment infrastructure snapshot (from atlas.yaml plan).
ALTER TABLE apps
	ADD COLUMN IF NOT EXISTS deployment_snapshot JSONB;
