-- App listen port is declared in atlas.yaml; drop the unused control-plane column.
ALTER TABLE apps DROP COLUMN IF EXISTS port;
