export type App = {
  id: string
  name: string
  port: number
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

export type Build = {
  id: string
  app_id: string
  status: 'pending' | 'running' | 'succeeded' | 'failed' | string
  image: string
  created_at: string
  updated_at: string
}

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
  updateAppPort: (id: string, port: number) =>
    request<App>(`/apps/${id}`, { method: 'PATCH', body: JSON.stringify({ port }) }),
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
  getInfrastructure: (id: string) =>
    request<Infrastructure>(`/apps/${id}/infrastructure`),

  getStatus: () => request<Status>('/status'),

  listGitHubInstallations: () => request<GitHubInstallation[]>('/github/installations'),
  syncGitHubInstallations: () =>
    request<GitHubInstallation[]>('/github/installations/sync', { method: 'POST' }),
  listGitHubRepos: (installationId: number) =>
    request<GitHubRepo[]>(`/github/installations/${installationId}/repos`),

  githubInstallURL: (returnPath: string) =>
    `/auth/github/install?return=${encodeURIComponent(returnPath)}`,
}
