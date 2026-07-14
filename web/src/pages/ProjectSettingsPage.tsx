import { useEffect, useState } from 'react'
import { Link, NavLink, useNavigate, useParams } from 'react-router-dom'
import { api, type App, type Repo, type Status } from '../api/client'

export function ProjectSettingsPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [app, setApp] = useState<App | null>(null)
  const [repo, setRepo] = useState<Repo | null>(null)
  const [status, setStatus] = useState<Status | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [confirm, setConfirm] = useState('')
  const [port, setPort] = useState('80')

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const [a, r, st] = await Promise.all([
          api.getApp(id),
          api.getRepo(id),
          api.getStatus(),
        ])
        if (!cancelled) {
          setApp(a)
          setRepo(r)
          setStatus(st)
          setPort(String(a.port || 80))
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Failed to load')
      }
    })()
    return () => {
      cancelled = true
    }
  }, [id])

  async function savePort() {
    const n = Number(port)
    if (!Number.isInteger(n) || n < 1 || n > 65535) {
      setError('Port must be an integer between 1 and 65535')
      return
    }
    setBusy(true)
    setError('')
    try {
      const updated = await api.updateAppPort(id, n)
      setApp(updated)
      setPort(String(updated.port))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to update port')
    } finally {
      setBusy(false)
    }
  }

  async function unlink() {
    setBusy(true)
    setError('')
    try {
      await api.unlinkRepo(id)
      setRepo(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to unlink')
    } finally {
      setBusy(false)
    }
  }

  async function remove() {
    if (!app || confirm !== app.name) {
      setError(`Type ${app?.name ?? 'the project name'} to confirm delete`)
      return
    }
    setBusy(true)
    setError('')
    try {
      await api.deleteApp(id)
      navigate('/')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
      setBusy(false)
    }
  }

  if (!app && !error) return <p className="muted">Loading…</p>
  if (!app) return <p className="error">{error}</p>

  const githubApp = status?.github_app_configured
  const returnPath = `/projects/${id}/settings`

  return (
    <>
      <div className="page-header">
        <div>
          <h1>{app.name}</h1>
          <p>Settings</p>
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
        <h2>Repository</h2>
        {repo ? (
          <div className="panel-row">
            <div>
              <div className="mono">{repo.github_full_name || repo.url}</div>
              <div className="muted" style={{ marginTop: '0.35rem' }}>
                Unlinking stops webhook rebuilds. Running workloads stay until you delete the project.
              </div>
            </div>
            <button className="btn btn-secondary" type="button" onClick={() => void unlink()} disabled={busy}>
              Unlink
            </button>
          </div>
        ) : (
          <p className="muted">
            No repository linked. <Link to={`/projects/${id}`}>Link one on Overview</Link>.
          </p>
        )}
      </div>

      <div className="panel">
        <h2>Container port</h2>
        <p className="muted" style={{ margin: 0 }}>
          Port your process listens on inside the container (Atlas Service stays on 80 and forwards here).
          Redeploy after changing this.
        </p>
        <div className="field" style={{ maxWidth: 200, marginTop: '0.75rem' }}>
          <label htmlFor="container-port">Port</label>
          <input
            id="container-port"
            type="number"
            min={1}
            max={65535}
            value={port}
            onChange={(e) => setPort(e.target.value)}
          />
        </div>
        <button
          className="btn btn-secondary"
          type="button"
          style={{ marginTop: '0.85rem' }}
          onClick={() => void savePort()}
          disabled={busy || String(app.port) === port}
        >
          Save port
        </button>
      </div>

      {githubApp ? (
        <div className="panel">
          <h2>GitHub App</h2>
          <p className="muted" style={{ margin: 0 }}>
            Atlas receives push webhooks through the installed GitHub App
            {status?.github_app_slug ? (
              <>
                {' '}
                (<code className="mono">{status.github_app_slug}</code>)
              </>
            ) : null}
            . Reconnect if repository access was revoked.
          </p>
          {(status?.github_installations?.length ?? 0) > 0 && (
            <ul style={{ margin: '0.75rem 0 0', paddingLeft: '1.25rem' }}>
              {status?.github_installations?.map((inst) => (
                <li key={inst.id} className="mono">
                  {inst.account_login} ({inst.account_type})
                </li>
              ))}
            </ul>
          )}
          <a
            className="btn btn-secondary"
            href={api.githubInstallURL(returnPath)}
            style={{ marginTop: '0.85rem', display: 'inline-block' }}
          >
            {status?.github_installations?.length ? 'Manage GitHub access' : 'Connect GitHub'}
          </a>
        </div>
      ) : (
        <div className="panel">
          <h2>Webhooks</h2>
          <p className="muted" style={{ margin: 0 }}>
            In GitHub → Settings → Webhooks, add a push webhook:
          </p>
          <p className="mono" style={{ margin: '0.75rem 0 0' }}>
            {status?.webhook_public_url || 'https://hooks.edwardscott.dev/webhooks/github'}
          </p>
          <p className="muted" style={{ margin: '0.75rem 0 0' }}>
            Content type <code className="mono">application/json</code>, secret ={' '}
            <code className="mono">ATLAS_WEBHOOK_SECRET</code>, events: Push. Or configure the GitHub App env vars
            for automatic webhooks.
          </p>
        </div>
      )}

      <div className="panel danger-zone">
        <h2>Delete project</h2>
        <p className="muted">
          Removes the Deployment, Service, and Ingress from the cluster, then deletes the project record.
        </p>
        <div className="field" style={{ maxWidth: 360, marginTop: '0.75rem' }}>
          <label htmlFor="confirm">Type {app.name} to confirm</label>
          <input
            id="confirm"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            placeholder={app.name}
          />
        </div>
        <button
          className="btn btn-danger"
          type="button"
          style={{ marginTop: '0.85rem' }}
          onClick={() => void remove()}
          disabled={busy || confirm !== app.name}
        >
          Delete project
        </button>
      </div>
    </>
  )
}
