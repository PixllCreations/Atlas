ALTER TABLE apps
    ADD COLUMN IF NOT EXISTS port INTEGER NOT NULL DEFAULT 80;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'apps_port_range'
    ) THEN
        ALTER TABLE apps
            ADD CONSTRAINT apps_port_range CHECK (port >= 1 AND port <= 65535);
    END IF;
END $$;
