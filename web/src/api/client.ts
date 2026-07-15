export type App = {
  id: string
  name: string
  created_at: string
  updated_at: string
}

export type Repo = {
  url: string
  provider: string
  branch: string
  github_full_name?: string
  installation_id?: number
}

export type BuildPhase = 'queued' | 'cloning' | 'building' | 'pushing' | 'deploying' | string

export type Build = {
  id: string
  app_id: string
  status: 'pending' | 'running' | 'succeeded' | 'failed' | string
  phase: BuildPhase
  image: string
  created_at: string
  updated_at: string
}

export type BuildLogsSnapshot = {
  build_id: string
  status: string
  phase: string
  log: string
  offset: number
}

export const BUILD_PHASES: { id: BuildPhase; label: string }[] = [
  { id: 'queued', label: 'Queued' },
  { id: 'cloning', label: 'Cloning' },
  { id: 'building', label: 'Building' },
  { id: 'pushing', label: 'Pushing' },
  { id: 'deploying', label: 'Deploying' },
]

export type GitHubInstallation = {
  id: number
  account_login: string
  account_type: string
}

export type GitHubRepo = {
  id: number
  full_name: string
  html_url: string
  private: boolean
  default_branch: string
}

export type Status = {
  ok: boolean
  port: string
  ingress_domain: string
  registry_set: boolean
  namespace: string
  kubernetes: boolean
  webhook_configured: boolean
  webhook_public_url: string
  github_app_configured: boolean
  github_app_slug?: string
  github_installations?: GitHubInstallation[]
}

export type InfrastructureDependency = {
  name: string
  type: string
  endpoint?: string
  status?: string
}

export type Infrastructure = {
  namespace: string
  host?: string
  app_name: string
  app_port: number
  dependencies: InfrastructureDependency[]
}

export type Workload = {
  name: string
  component: string
  type?: string
  ready: boolean
  replicas?: number
  source?: string
}

export type TriggerBuildResponse = {
  status: string
  app_id: string
  build_id: string
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
  })
  if (res.status === 204) {
    return undefined as T
  }
  const text = await res.text()
  const data = text ? JSON.parse(text) : null
  if (!res.ok) {
    const message = data?.error ?? res.statusText
    throw new Error(message)
  }
  return data as T
}

export const api = {
  listApps: () => request<App[]>('/apps'),
  getApp: (id: string) => request<App>(`/apps/${id}`),
  createApp: (name: string) =>
    request<App>('/apps', { method: 'POST', body: JSON.stringify({ name }) }),
  deleteApp: (id: string) => request<void>(`/apps/${id}`, { method: 'DELETE' }),

  getRepo: async (id: string): Promise<Repo | null> => {
    const res = await fetch(`/apps/${id}/repo`)
    if (res.status === 404) return null
    const text = await res.text()
    const data = text ? JSON.parse(text) : null
    if (!res.ok) throw new Error(data?.error ?? res.statusText)
    return data as Repo
  },
  linkRepo: (id: string, body: { url?: string; branch: string; github_full_name?: string; installation_id?: number }) =>
    request<Repo>(`/apps/${id}/repo`, {
      method: 'PUT',
      body: JSON.stringify({ ...body, provider: 'github' }),
    }),
  linkRepoURL: (id: string, url: string, branch: string) =>
    request<Repo>(`/apps/${id}/repo`, {
      method: 'PUT',
      body: JSON.stringify({ url, branch, provider: 'github' }),
    }),
  unlinkRepo: (id: string) =>
    request<void>(`/apps/${id}/repo`, { method: 'DELETE' }),

  listBuilds: (id: string) => request<Build[]>(`/apps/${id}/builds`),
  triggerBuild: (id: string) =>
    request<TriggerBuildResponse>(`/apps/${id}/builds`, { method: 'POST' }),
  getBuildLogs: (appId: string, buildId: string) =>
    request<BuildLogsSnapshot>(`/apps/${appId}/builds/${buildId}/logs`),
  streamBuildLogs: (
    appId: string,
    buildId: string,
    handlers: {
      onLog?: (chunk: string, offset: number) => void
      onStatus?: (status: string, phase: string) => void
      onDone?: (status: string) => void
      onError?: (message: string) => void
    },
    offset = 0,
  ) => {
    const params = new URLSearchParams({ follow: '1' })
    if (offset > 0) params.set('offset', String(offset))
    const es = new EventSource(`/apps/${appId}/builds/${buildId}/logs?${params}`)
    let finished = false
    es.addEventListener('log', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as { chunk?: string; offset?: number }
        handlers.onLog?.(data.chunk ?? '', data.offset ?? 0)
      } catch {
        /* ignore */
      }
    })
    es.addEventListener('status', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as { status?: string; phase?: string }
        handlers.onStatus?.(data.status ?? '', data.phase ?? '')
      } catch {
        /* ignore */
      }
    })
    es.addEventListener('done', (ev) => {
      finished = true
      try {
        const data = JSON.parse((ev as MessageEvent).data) as { status?: string }
        handlers.onDone?.(data.status ?? '')
      } catch {
        /* ignore */
      }
      es.close()
    })
    es.onerror = () => {
      if (!finished) handlers.onError?.('log stream interrupted')
      es.close()
    }
    return () => es.close()
  },
  getInfrastructure: (id: string) =>
    request<Infrastructure>(`/apps/${id}/infrastructure`),
  listWorkloads: (id: string) => request<Workload[]>(`/apps/${id}/workloads`),
  streamWorkloadLogs: (
    appId: string,
    workload: string,
    handlers: {
      onLog?: (chunk: string) => void
      onStatus?: (info: { workload: string; pod: string; container: string }) => void
      onDone?: () => void
      onError?: (message: string) => void
    },
    tailLines = 200,
  ) => {
    const params = new URLSearchParams({ follow: '1', tailLines: String(tailLines) })
    const es = new EventSource(`/apps/${appId}/workloads/${encodeURIComponent(workload)}/logs?${params}`)
    let finished = false
    es.addEventListener('log', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as { chunk?: string }
        handlers.onLog?.(data.chunk ?? '')
      } catch {
        /* ignore */
      }
    })
    es.addEventListener('status', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as {
          workload?: string
          pod?: string
          container?: string
        }
        handlers.onStatus?.({
          workload: data.workload ?? workload,
          pod: data.pod ?? '',
          container: data.container ?? '',
        })
      } catch {
        /* ignore */
      }
    })
    es.addEventListener('done', () => {
      finished = true
      handlers.onDone?.()
      es.close()
    })
    es.addEventListener('error', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as { error?: string }
        if (data.error) handlers.onError?.(data.error)
      } catch {
        /* EventSource connection errors have no JSON payload */
      }
    })
    es.onerror = () => {
      if (!finished) handlers.onError?.('runtime log stream interrupted')
      es.close()
    }
    return () => es.close()
  },

  getStatus: () => request<Status>('/status'),

  listGitHubInstallations: () => request<GitHubInstallation[]>('/github/installations'),
  syncGitHubInstallations: () =>
    request<GitHubInstallation[]>('/github/installations/sync', { method: 'POST' }),
  listGitHubRepos: (installationId: number) =>
    request<GitHubRepo[]>(`/github/installations/${installationId}/repos`),

  githubInstallURL: (returnPath: string) =>
    `/auth/github/install?return=${encodeURIComponent(returnPath)}`,
}
