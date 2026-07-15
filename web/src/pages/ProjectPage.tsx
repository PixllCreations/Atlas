import { useCallback, useEffect, useMemo, useRef, useState, type SubmitEvent } from 'react'
import { Link, NavLink, useParams } from 'react-router-dom'
import {
  api,
  type App,
  type Build,
  type GitHubRepo,
  type Infrastructure,
  type Repo,
  type Status,
  type Workload,
} from '../api/client'
import { BuildPhases } from '../components/BuildPhases'
import { formatTime, StatusBadge } from '../components/StatusBadge'
import { appPublicURL } from '../lib/urls'

type LogTab = 'build' | 'runtime'

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
  const [logText, setLogText] = useState('')
  const [runtimeLogText, setRuntimeLogText] = useState('')
  const [expandedBuildId, setExpandedBuildId] = useState<string | null>(null)
  const [logTab, setLogTab] = useState<LogTab>('build')
  const [workloads, setWorkloads] = useState<Workload[]>([])
  const [selectedWorkload, setSelectedWorkload] = useState('app')
  const [runtimeMeta, setRuntimeMeta] = useState('')
  const [runtimeError, setRuntimeError] = useState('')
  const [now, setNow] = useState(() => Date.now())
  const logEndRef = useRef<HTMLPreElement>(null)
  const runtimeLogEndRef = useRef<HTMLPreElement>(null)
  const autoExpandedRef = useRef<string | null>(null)

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

  const activeBuild = useMemo(
    () => builds.find((b) => b.status === 'pending' || b.status === 'running') ?? null,
    [builds],
  )
  const expandedBuild = useMemo(
    () => (expandedBuildId ? (builds.find((b) => b.id === expandedBuildId) ?? null) : null),
    [builds, expandedBuildId],
  )

  // Auto-expand the in-progress deploy (once per build id).
  useEffect(() => {
    if (!activeBuild) return
    if (autoExpandedRef.current === activeBuild.id) return
    autoExpandedRef.current = activeBuild.id
    setExpandedBuildId(activeBuild.id)
    setLogTab('build')
  }, [activeBuild])

  useEffect(() => {
    if (!activeBuild) return
    const t = setInterval(() => {
      void refresh().catch(() => {})
    }, 2000)
    return () => clearInterval(t)
  }, [activeBuild, refresh])

  useEffect(() => {
    if (!activeBuild) return
    const t = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(t)
  }, [activeBuild])

  useEffect(() => {
    if (!expandedBuild || logTab !== 'build') {
      if (!expandedBuild) setLogText('')
      return
    }
    let cancelled = false
    let stopStream: (() => void) | undefined
    ;(async () => {
      try {
        const snap = await api.getBuildLogs(id, expandedBuild.id)
        if (cancelled) return
        setLogText(snap.log)
        if (expandedBuild.status === 'pending' || expandedBuild.status === 'running') {
          stopStream = api.streamBuildLogs(
            id,
            expandedBuild.id,
            {
              onLog: (chunk) => {
                setLogText((prev) => prev + chunk)
              },
              onDone: () => {
                void refresh().catch(() => {})
              },
            },
            snap.offset,
          )
        }
      } catch {
        /* ignore */
      }
    })()
    return () => {
      cancelled = true
      stopStream?.()
    }
  }, [id, expandedBuild?.id, expandedBuild?.status, logTab, refresh])

  useEffect(() => {
    if (!expandedBuildId || logTab !== 'runtime') {
      return
    }
    let cancelled = false
    ;(async () => {
      try {
        const list = await api.listWorkloads(id)
        if (cancelled) return
        setWorkloads(list)
        setSelectedWorkload((prev) => {
          if (list.some((w) => w.name === prev)) return prev
          return list[0]?.name ?? 'app'
        })
      } catch (e) {
        if (!cancelled) {
          setRuntimeError(e instanceof Error ? e.message : 'Failed to list workloads')
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [id, expandedBuildId, logTab])

  useEffect(() => {
    if (!expandedBuildId || logTab !== 'runtime' || !selectedWorkload) {
      if (logTab !== 'runtime') {
        setRuntimeLogText('')
        setRuntimeMeta('')
        setRuntimeError('')
      }
      return
    }
    let cancelled = false
    let stopStream: (() => void) | undefined
    setRuntimeLogText('')
    setRuntimeMeta('')
    setRuntimeError('')
    ;(async () => {
      try {
        const res = await fetch(
          `/apps/${id}/workloads/${encodeURIComponent(selectedWorkload)}/logs?tailLines=200`,
        )
        const text = await res.text()
        const data = text ? JSON.parse(text) : null
        if (cancelled) return
        if (!res.ok) {
          setRuntimeError(data?.error ?? res.statusText)
          return
        }
        // Stream live from the same point (includes recent history + follow).
        stopStream = api.streamWorkloadLogs(id, selectedWorkload, {
          onStatus: (info) => {
            setRuntimeMeta(`${info.pod} · ${info.container}`)
          },
          onLog: (chunk) => {
            setRuntimeLogText((prev) => prev + chunk)
          },
          onError: (message) => {
            setRuntimeError(message)
          },
        })
      } catch (e) {
        if (!cancelled) {
          setRuntimeError(e instanceof Error ? e.message : 'Failed to load runtime logs')
        }
      }
    })()
    return () => {
      cancelled = true
      stopStream?.()
    }
  }, [id, expandedBuildId, logTab, selectedWorkload])

  useEffect(() => {
    logEndRef.current?.scrollTo({ top: logEndRef.current.scrollHeight })
  }, [logText])

  useEffect(() => {
    runtimeLogEndRef.current?.scrollTo({ top: runtimeLogEndRef.current.scrollHeight })
  }, [runtimeLogText])

  function toggleExpand(buildId: string) {
    setExpandedBuildId((prev) => (prev === buildId ? null : buildId))
    setLogTab('build')
  }

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
      const res = await api.triggerBuild(id)
      autoExpandedRef.current = res.build_id
      setExpandedBuildId(res.build_id)
      setLogTab('build')
      setLogText('')
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
  const deployDisabled = busy || !repo || !!activeBuild
  const elapsed =
    expandedBuild && (expandedBuild.status === 'pending' || expandedBuild.status === 'running')
      ? Math.max(0, Math.floor((now - new Date(expandedBuild.created_at).getTime()) / 1000))
      : null

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
          <button
            className="btn btn-primary"
            type="button"
            onClick={() => void deploy()}
            disabled={deployDisabled}
          >
            {activeBuild ? 'Deploying…' : busy ? 'Working…' : 'Deploy'}
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
              <li key={d.name} className="build-item dep-item">
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
            {builds.map((b) => {
              const open = expandedBuildId === b.id
              return (
                <li key={b.id} className={`build-card${open ? ' build-card-open' : ''}`}>
                  <button
                    type="button"
                    className="build-card-toggle"
                    aria-expanded={open}
                    onClick={() => toggleExpand(b.id)}
                  >
                    <StatusBadge status={b.status} />
                    <div className="build-card-meta">
                      <div className="mono">{b.image || '—'}</div>
                      <div className="muted" style={{ fontSize: '0.8rem' }}>
                        {formatTime(b.created_at)} · <span className="mono">{b.id.slice(0, 8)}</span>
                      </div>
                    </div>
                    <BuildPhases build={b} compact />
                    <span className={`build-chevron${open ? ' open' : ''}`} aria-hidden>
                      ▾
                    </span>
                  </button>
                  {open ? (
                    <div className="build-card-body">
                      <BuildPhases
                        build={b}
                        elapsedSeconds={expandedBuild?.id === b.id ? elapsed : null}
                      />
                      <div className="log-tabs" role="tablist">
                        <button
                          type="button"
                          role="tab"
                          aria-selected={logTab === 'build'}
                          className={logTab === 'build' ? 'active' : undefined}
                          onClick={() => setLogTab('build')}
                        >
                          Build
                        </button>
                        <button
                          type="button"
                          role="tab"
                          aria-selected={logTab === 'runtime'}
                          className={logTab === 'runtime' ? 'active' : undefined}
                          onClick={() => setLogTab('runtime')}
                        >
                          Runtime
                        </button>
                      </div>
                      {logTab === 'build' ? (
                        <pre className="build-log" ref={logEndRef}>
                          {logText || <span className="muted">No log output yet.</span>}
                        </pre>
                      ) : (
                        <div className="runtime-logs">
                          <div className="runtime-logs-toolbar">
                            <label htmlFor={`workload-${b.id}`}>
                              Service
                              <select
                                id={`workload-${b.id}`}
                                value={selectedWorkload}
                                onChange={(e) => setSelectedWorkload(e.target.value)}
                              >
                                {(workloads.length > 0
                                  ? workloads
                                  : [{ name: 'app', component: 'application', ready: false }]
                                ).map((w) => (
                                  <option key={w.name} value={w.name}>
                                    {w.name}
                                    {w.type && w.type !== 'app' ? ` (${w.type})` : ''}
                                    {w.ready ? '' : w.source === 'live' ? ' · not ready' : ''}
                                  </option>
                                ))}
                              </select>
                            </label>
                            {runtimeMeta ? (
                              <span className="mono muted runtime-meta">{runtimeMeta}</span>
                            ) : null}
                          </div>
                          {runtimeError ? <p className="error">{runtimeError}</p> : null}
                          <pre className="build-log" ref={runtimeLogEndRef}>
                            {runtimeLogText || (
                              <span className="muted">Waiting for runtime logs…</span>
                            )}
                          </pre>
                        </div>
                      )}
                    </div>
                  ) : null}
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </>
  )
}
