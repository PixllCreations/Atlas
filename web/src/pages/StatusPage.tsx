import { useEffect, useState } from 'react'
import { api, type Status } from '../api/client'

export function StatusPage() {
  const [status, setStatus] = useState<Status | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const s = await api.getStatus()
        if (!cancelled) setStatus(s)
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Failed to load status')
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
          <h1>Status</h1>
          <p>Runtime hints for this Atlas instance.</p>
        </div>
      </div>

      {error && <p className="error">{error}</p>}
      {!status && !error && <p className="muted">Loading…</p>}

      {status && (
        <div className="panel">
          <div className="status-grid">
            <div className="status-tile">
              <div className="label">API</div>
              <div className="value">{status.ok ? 'Healthy' : 'Down'}</div>
            </div>
            <div className="status-tile">
              <div className="label">Port</div>
              <div className="value mono">{status.port}</div>
            </div>
            <div className="status-tile">
              <div className="label">Kubernetes</div>
              <div className="value">{status.kubernetes ? 'Connected' : 'Unavailable'}</div>
            </div>
            <div className="status-tile">
              <div className="label">Registry</div>
              <div className="value">{status.registry_set ? 'Configured' : 'Not set'}</div>
            </div>
            <div className="status-tile">
              <div className="label">Namespace</div>
              <div className="value mono">{status.namespace}</div>
            </div>
            <div className="status-tile">
              <div className="label">Ingress domain</div>
              <div className="value mono">{status.ingress_domain || '—'}</div>
            </div>
            <div className="status-tile">
              <div className="label">GitHub App</div>
              <div className="value">{status.github_app_configured ? 'Configured' : 'Not set'}</div>
            </div>
            <div className="status-tile">
              <div className="label">Webhook secret</div>
              <div className="value">{status.webhook_configured ? 'Set' : 'Missing'}</div>
            </div>
            <div className="status-tile">
              <div className="label">Webhook URL</div>
              <div className="value mono" style={{ fontSize: '0.85rem', wordBreak: 'break-all' }}>
                {status.webhook_public_url || '—'}
              </div>
            </div>
          </div>
          {status.github_installations && status.github_installations.length > 0 && (
            <div className="hint" style={{ marginTop: '1rem' }}>
              GitHub installations:{' '}
              {status.github_installations.map((i) => `${i.account_login} (${i.account_type})`).join(', ')}
            </div>
          )}
          <div className="hint">
            Console (private): <code>http://&lt;atlas-host&gt;:{status.port}</code> over Tailscale. Public apps:{' '}
            <code>https://&lt;app&gt;.{status.ingress_domain || 'edwardscott.dev'}</code> via Cloudflare Tunnel.
          </div>
        </div>
      )}
    </>
  )
}
