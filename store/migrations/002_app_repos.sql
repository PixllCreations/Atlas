CREATE TABLE app_repos (
    app_id     UUID PRIMARY KEY REFERENCES apps(id) ON DELETE CASCADE,
    url        TEXT NOT NULL,
    provider   TEXT NOT NULL,
    branch     TEXT NOT NULL DEFAULT 'main',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
