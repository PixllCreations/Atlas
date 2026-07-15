import { useCallback, useEffect, useState, type SubmitEvent } from 'react'
import { Link, NavLink, useParams } from 'react-router-dom'
import { api, type App, type Build, type GitHubRepo, type Infrastructure, type Repo, type Status } from '../api/client'
import { formatTime, StatusBadge } from '../components/StatusBadge'
import { appPublicURL } from '../lib/urls'

export function ProjectPage() {
  const { id = '' } = useParams()
  const [app, setApp] = useState<App | null>(null)
  const [repo, setRepo] = useState<Repo | null>(null)
  const [builds, setBuilds] = useState<Build[]>([])
  const [infra, setInfra] = useState<Infrastructure | null>(null)
  const [status, setStatus] = useState<Status | null>(null)
  const [url, setUrl] = useState('')
  const [branch, setBranch] = useState('main')
  const [installationId, setInstallationId] = useState<number | null>(null)
  const [githubRepos, setGithubRepos] = useState<GitHubRepo[]>([])
  const [selectedRepo, setSelectedRepo] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const refresh = useCallback(async () => {
    const [a, r, b, st, inf] = await Promise.all([
      api.getApp(id),
      api.getRepo(id),
      api.listBuilds(id),
      api.getStatus(),
      api.getInfrastructure(id),
    ])
    setApp(a)
    setRepo(r)
    setBuilds(b)
    setStatus(st)
    setInfra(inf)
    if (r) {
      setUrl(r.url)
      setBranch(r.branch)
      if (r.installation_id) setInstallationId(r.installation_id)
      if (r.github_full_name) setSelectedRepo(r.github_full_name)
    } else if (st.github_installations?.length) {
      setInstallationId(st.github_installations[0].id)
    }
  }, [id])

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        await refresh()
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Failed to load')
      }
    })()
    return () => {
      cancelled = true
    }
  }, [refresh])

  useEffect(() => {
    if (!status?.github_app_configured || !installationId || repo) {
      setGithubRepos([])
      return
    }
    let cancelled = false
    ;(async () => {
      try {
        const repos = await api.listGitHubRepos(installationId)
        if (!cancelled) {
          setGithubRepos(repos)
          if (!selectedRepo && repos.length > 0) {
            setSelectedRepo(repos[0].full_name)
            if (repos[0].default_branch) setBranch(repos[0].default_branch)
          }
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Failed to load repositories')
      }
    })()
    return () => {
      cancelled = true
    }
  }, [status?.github_app_configured, installationId, repo, selectedRepo])

  useEffect(() => {
    const active = builds.some((b) => b.status === 'pending' || b.status === 'running')
    if (!active) return
    const t = setInterval(() => {
      void refresh().catch(() => {})
    }, 2500)
    return () => clearInterval(t)
  }, [builds, refresh])

  async function linkManualRepo(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const r = await api.linkRepoURL(id, url.trim(), branch.trim() || 'main')
      setRepo(r)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to link repo')
    } finally {
      setBusy(false)
    }
  }

  async function linkGitHubRepo(e: SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!installationId || !selectedRepo) return
    setBusy(true)
    setError('')
    try {
      const r = await api.linkRepo(id, {
        github_full_name: selectedRepo,
        installation_id: installationId,
        branch: branch.trim() || 'main',
      })
      setRepo(r)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to link repo')
    } finally {
      setBusy(false)
    }
  }

  async function deploy() {
    setBusy(true)
    setError('')
    try {
      await api.triggerBuild(id)
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to deploy')
    } finally {
      setBusy(false)
    }
  }

  if (!app && !error) {
    return <p className="muted">Loading…</p>
  }

  if (!app) {
    return <p className="error">{error || 'Project not found'}</p>
  }

  const openUrl = status?.ingress_domain
    ? appPublicURL(app.name, status.ingress_domain)
    : null
  const githubApp = status?.github_app_configured
  const installations = status?.github_installations ?? []
  const returnPath = `/projects/${id}`

  return (
    <>
      <div className="page-header">
        <div>
          <h1>{app.name}</h1>
          <p className="mono muted">{app.id}</p>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
          {openUrl && (
            <a className="btn btn-secondary" href={openUrl} target="_blank" rel="noreferrer">
              Open
            </a>
          )}
          <button className="btn btn-primary" type="button" onClick={() => void deploy()} disabled={busy || !repo}>
            {busy ? 'Working…' : 'Deploy'}
          </button>
        </div>
      </div>

      <div className="tabs">
        <NavLink to={`/projects/${id}`} end>
          Overview
        </NavLink>
        <NavLink to={`/projects/${id}/settings`}>Settings</NavLink>
      </div>

      {error && <p className="error">{error}</p>}

      <div className="panel">
        <h2>Source</h2>
        {repo ? (
          <div className="panel-row">
            <div>
              <div className="mono">{repo.github_full_name || repo.url}</div>
              <div className="muted" style={{ marginTop: '0.25rem' }}>
                Branch <span className="mono">{repo.branch}</span> · pushes rebuild automatically
              </div>
            </div>
            <Link className="btn btn-secondary" to={`/projects/${id}/settings`}>
              Manage
            </Link>
          </div>
        ) : githubApp ? (
          installations.length === 0 ? (
            <div className="panel-row">
              <div>
                <p className="muted" style={{ margin: 0 }}>
                  Connect GitHub to pick a repository. Atlas receives push webhooks through the GitHub App — no
                  manual webhook setup.
                </p>
              </div>
              <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
                <a className="btn btn-primary" href={api.githubInstallURL(returnPath)}>
                  Connect GitHub
                </a>
                <button
                  className="btn btn-secondary"
                  type="button"
                  disabled={busy}
                  onClick={() => {
                    void (async () => {
                      setBusy(true)
                      setError('')
                      try {
                        const insts = await api.syncGitHubInstallations()
                        if (insts.length === 0) {
                          setError('No GitHub App installations found. Click Connect GitHub first.')
                        } else {
                          await refresh()
                        }
                      } catch (err) {
                        setError(err instanceof Error ? err.message : 'Failed to sync GitHub')
                      } finally {
                        setBusy(false)
                      }
                    })()
                  }}
                >
                  Sync existing install
                </button>
              </div>
            </div>
          ) : (
            <form className="form" onSubmit={(e) => void linkGitHubRepo(e)}>
              {installations.length > 1 && (
                <div className="field">
                  <label htmlFor="installation">GitHub account</label>
                  <select
                    id="installation"
                    value={installationId ?? ''}
                    onChange={(e) => setInstallationId(Number(e.target.value))}
                  >
                    {installations.map((inst) => (
                      <option key={inst.id} value={inst.id}>
                        {inst.account_login} ({inst.account_type})
                      </option>
                    ))}
                  </select>
                </div>
              )}
              <div className="field">
                <label htmlFor="github-repo">Repository</label>
                <select
                  id="github-repo"
                  value={selectedRepo}
                  onChange={(e) => {
                    setSelectedRepo(e.target.value)
                    const match = githubRepos.find((r) => r.full_name === e.target.value)
                    if (match?.default_branch) setBranch(match.default_branch)
                  }}
                  required
                >
                  {githubRepos.length === 0 ? (
                    <option value="">Loading repositories…</option>
                  ) : (
                    githubRepos.map((r) => (
                      <option key={r.id} value={r.full_name}>
                        {r.full_name}
                        {r.private ? ' (private)' : ''}
                      </option>
                    ))
                  )}
                </select>
              </div>
              <div className="field">
                <label htmlFor="branch">Branch</label>
                <input id="branch" value={branch} onChange={(e) => setBranch(e.target.value)} placeholder="main" />
              </div>
              <button className="btn btn-primary" type="submit" disabled={busy || !selectedRepo}>
                Link repository
              </button>
            </form>
          )
        ) : (
          <form className="form" onSubmit={(e) => void linkManualRepo(e)}>
            <div className="field">
              <label htmlFor="repo-url">GitHub repository URL</label>
              <input
                id="repo-url"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="https://github.com/you/portfolio"
                required
              />
            </div>
            <div className="field">
              <label htmlFor="branch">Branch</label>
              <input id="branch" value={branch} onChange={(e) => setBranch(e.target.value)} placeholder="main" />
            </div>
            <button className="btn btn-primary" type="submit" disabled={busy}>
              Link repository
            </button>
          </form>
        )}
        {githubApp ? (
          <div className="hint">
            Push events arrive via the Atlas GitHub App
            {status?.github_app_slug ? (
              <>
                {' '}
                (<code>{status.github_app_slug}</code>)
              </>
            ) : null}
            . Reinstall from Settings if access was revoked.
          </div>
        ) : (
          <div className="hint">
            Configure a GitHub push webhook to <code>{status?.webhook_public_url || '/webhooks/github'}</code> with
            secret <code>ATLAS_WEBHOOK_SECRET</code>, or set up the GitHub App env vars for automatic webhooks.
          </div>
        )}
      </div>

      <div className="panel">
        <h2>Dependencies</h2>
        {!infra || infra.dependencies.length === 0 ? (
          <p className="muted">
            No managed dependencies. Declare them in <code>atlas.yaml</code> at the repository root.
          </p>
        ) : (
          <ul className="build-list">
            {infra.dependencies.map((d) => (
              <li key={d.name} className="build-item">
                <div>
                  <div style={{ textTransform: 'capitalize' }}>{d.type}</div>
                  <div className="muted" style={{ marginTop: '0.25rem' }}>
                    Type <span className="mono">{d.type}</span>
                    {d.endpoint ? (
                      <>
                        {' '}
                        · Endpoint <span className="mono">{d.endpoint}</span>
                      </>
                    ) : null}
                    {d.status ? (
                      <>
                        {' '}
                        · {d.status}
                      </>
                    ) : null}
                  </div>
                </div>
                <span className="mono muted">{d.name}</span>
              </li>
            ))}
          </ul>
        )}
        {infra?.app_port ? (
          <div className="hint">
            App listens on port <span className="mono">{infra.app_port}</span>
            {infra.namespace ? (
              <>
                {' '}
                · namespace <span className="mono">{infra.namespace}</span>
              </>
            ) : null}
          </div>
        ) : null}
      </div>

      <div className="panel">
        <h2>Deployments</h2>
        {builds.length === 0 ? (
          <p className="muted">No builds yet. Link a repo and click Deploy.</p>
        ) : (
          <ul className="build-list">
            {builds.map((b) => (
              <li key={b.id} className="build-item">
                <StatusBadge status={b.status} />
                <div>
                  <div className="mono">{b.image || '—'}</div>
                  <div className="muted" style={{ fontSize: '0.8rem' }}>
                    {formatTime(b.created_at)}
                  </div>
                </div>
                <span className="mono muted">{b.id.slice(0, 8)}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </>
  )
}
