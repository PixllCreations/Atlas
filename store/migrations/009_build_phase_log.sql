-- Deploy phase and accumulated build log for console UX / SSE.
ALTER TABLE builds
	ADD COLUMN IF NOT EXISTS phase TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS log TEXT NOT NULL DEFAULT '';
