CREATE TABLE IF NOT EXISTS github_installations (
    id              BIGINT PRIMARY KEY,
    account_login   TEXT NOT NULL,
    account_type    TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE app_repos
    ADD COLUMN IF NOT EXISTS github_repo_id   BIGINT,
    ADD COLUMN IF NOT EXISTS github_full_name TEXT,
    ADD COLUMN IF NOT EXISTS installation_id  BIGINT REFERENCES github_installations(id);

CREATE UNIQUE INDEX IF NOT EXISTS app_repos_github_repo_branch
    ON app_repos (github_repo_id, branch)
    WHERE github_repo_id IS NOT NULL;
