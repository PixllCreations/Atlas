import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type App, type Build, type Status } from '../api/client'
import { StatusBadge } from '../components/StatusBadge'
import { appPublicURL } from '../lib/urls'

type ProjectRow = App & { latest?: Build }

export function ProjectsPage() {
  const [rows, setRows] = useState<ProjectRow[]>([])
  const [status, setStatus] = useState<Status | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const [apps, st] = await Promise.all([api.listApps(), api.getStatus()])
        if (cancelled) return
        setStatus(st)
        const withBuilds = await Promise.all(
          apps.map(async (app) => {
            try {
              const builds = await api.listBuilds(app.id)
              return { ...app, latest: builds[0] }
            } catch {
              return app
            }
          }),
        )
        if (!cancelled) setRows(withBuilds)
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Failed to load')
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <>
      <div className="page-header">
        <div>
          <h1>Projects</h1>
          <p>Deploy from GitHub to your cluster.</p>
        </div>
        <Link className="btn btn-primary" to="/new">
          New project
        </Link>
      </div>

      {error && <p className="error">{error}</p>}
      {loading && <p className="muted">Loading…</p>}

      {!loading && rows.length === 0 && (
        <div className="empty">
          <p>No projects yet.</p>
          <p>
            <Link to="/new">Create your first project</Link>
          </p>
        </div>
      )}

      <div className="grid">
        {rows.map((app) => {
          const url = status?.ingress_domain
            ? appPublicURL(app.name, status.ingress_domain)
            : null
          return (
            <Link key={app.id} className="project-card" to={`/projects/${app.id}`}>
              <h2>{app.name}</h2>
              <div className="project-meta">
                {app.latest ? (
                  <StatusBadge status={app.latest.status} />
                ) : (
                  <span className="muted">No deploys</span>
                )}
                {url && <span className="mono">{url.replace(/^https?:\/\//, '')}</span>}
              </div>
            </Link>
          )
        })}
      </div>
    </>
  )
}
