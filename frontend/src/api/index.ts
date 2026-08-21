import { baseUrl, request } from './client'
import type {
  AlertEvent,
  AlertRule,
  AlertRulePayload,
  ApiKey,
  ApiKeyPayload,
  AppAction,
  AppLeftover,
  BackupObject,
  BackupPlan,
  BackupPlanPayload,
  BackupRun,
  ArchiveFormat,
  ArchiveFormats,
  ExtractResult,
  SourceDeployResult,
  AuditLog,
  CatalogResult,
  ComposeStatus,
  CreatedApiKey,
  DatabaseInfo,
  DatabaseServer,
  DatabaseServerPayload,
  DatabaseTable,
  DatabaseUser,
  DatabaseUserPayload,
  InstallPayload,
  InstalledApp,
  CertPayload,
  Certificate,
  DiskReport,
  DiskUsage,
  CronJob,
  CronJobPayload,
  CronRun,
  DockerAction,
  DockerContainer,
  DockerImage,
  DockerNetwork,
  DockerStats,
  DockerStatus,
  DockerVolume,
  FirewallRule,
  FirewallRulePayload,
  FirewallStatus,
  FileContent,
  FileInfo,
  FileList,
  HistorySample,
  LoginLog,
  LogChunk,
  LogSource,
  Overview,
  NotifyChannel,
  NotifyChannelPayload,
  Node,
  NodePayload,
  PluginList,
  PortListener,
  ProcessList,
  PageResult,
  PanelSettingsInfo,
  PanelSettings,
  PanelSettingsUpdate,
  QueryResult,
  Role,
  ServiceAction,
  ServiceManagerStatus,
  Session,
  Snapshot,
  SshKey,
  SystemAccount,
  SystemAccountPayload,
  DomainReport,
  HealthReport,
  RemoteResult,
  RewritePreset,
  SecurityOverview,
  TrafficReport,
  SystemService,
  TokenPair,
  UptimeCheck,
  UptimeMonitor,
  UptimeMonitorPayload,
  TotpSetup,
  User,
  WebServerStatus,
  Website,
  WebsitePayload,
} from './types'

/** Các endpoint xác thực. */
export const authApi = {
  login: (username: string, password: string, totpCode?: string) =>
    request<TokenPair>('/api/v1/auth/login', {
      method: 'POST',
      body: { username, password, totpCode },
      skipRefresh: true,
    }),

  refresh: (refreshToken: string) =>
    request<TokenPair>('/api/v1/auth/refresh', {
      method: 'POST',
      body: { refreshToken },
      skipRefresh: true,
    }),

  logout: (refreshToken: string) =>
    request<void>('/api/v1/auth/logout', { method: 'POST', body: { refreshToken } }),

  me: () => request<User>('/api/v1/auth/me'),

  changePassword: (currentPassword: string, newPassword: string) =>
    request<void>('/api/v1/auth/password', {
      method: 'POST',
      body: { currentPassword, newPassword },
    }),

  sessions: () => request<Session[]>('/api/v1/auth/sessions'),

  revokeSession: (id: number) =>
    request<void>(`/api/v1/auth/sessions/${id}`, { method: 'DELETE' }),

  beginTotp: () => request<TotpSetup>('/api/v1/auth/totp/setup', { method: 'POST' }),

  confirmTotp: (code: string) =>
    request<void>('/api/v1/auth/totp/confirm', { method: 'POST', body: { code } }),

  disableTotp: (password: string) =>
    request<void>('/api/v1/auth/totp/disable', { method: 'POST', body: { password } }),
}

/** Các endpoint giám sát tài nguyên. */
export const monitorApi = {
  overview: () => request<Overview>('/api/v1/monitor/overview'),
  current: () => request<Snapshot>('/api/v1/monitor/current'),
  history: (window = '1h') =>
    request<HistorySample[]>(`/api/v1/monitor/history?window=${encodeURIComponent(window)}`),
}

/** Các endpoint quản lý người dùng. */
export const userApi = {
  list: () => request<User[]>('/api/v1/users'),

  create: (payload: {
    username: string
    password: string
    email?: string
    role: Role
    language?: string
  }) => request<User>('/api/v1/users', { method: 'POST', body: payload }),

  update: (
    id: number,
    payload: { email?: string; role?: Role; active?: boolean },
  ) => request<User>(`/api/v1/users/${id}`, { method: 'PATCH', body: payload }),

  remove: (id: number) => request<void>(`/api/v1/users/${id}`, { method: 'DELETE' }),

  resetPassword: (id: number, newPassword: string) =>
    request<void>(`/api/v1/users/${id}/password`, { method: 'POST', body: { newPassword } }),

  updatePreferences: (payload: { language?: string; theme?: string }) =>
    request<User>('/api/v1/users/me/preferences', { method: 'PATCH', body: payload }),
}

/** Các endpoint quản lý tệp. */
export const fileApi = {
  list: (path: string) => request<FileList>(`/api/v1/files?path=${encodeURIComponent(path)}`),

  read: (path: string) =>
    request<FileContent>(`/api/v1/files/content?path=${encodeURIComponent(path)}`),

  write: (path: string, content: string) =>
    request<void>('/api/v1/files/content', { method: 'PUT', body: { path, content } }),

  stat: (path: string) => request<FileInfo>(`/api/v1/files/stat?path=${encodeURIComponent(path)}`),

  mkdir: (path: string) => request<void>('/api/v1/files/mkdir', { method: 'POST', body: { path } }),

  remove: (paths: string[]) =>
    request<void>('/api/v1/files/remove', { method: 'POST', body: { paths } }),

  move: (from: string, to: string) =>
    request<void>('/api/v1/files/move', { method: 'POST', body: { from, to } }),

  chmod: (path: string, mode: string) =>
    request<void>('/api/v1/files/chmod', { method: 'POST', body: { path, mode } }),

  compress: (paths: string[], target: string, format: ArchiveFormat) =>
    request<void>('/api/v1/files/compress', { method: 'POST', body: { paths, target, format } }),

  extract: (path: string, target: string) =>
    request<ExtractResult>('/api/v1/files/extract', { method: 'POST', body: { path, target } }),

  formats: () => request<ArchiveFormats>('/api/v1/files/formats'),

  /**
   * Xin vé tải xuống rồi trả về URL dùng được cho thẻ <a>.
   *
   * Trình duyệt không gửi được header Authorization khi điều hướng tới URL tải
   * tệp, nên phải có thứ gì đó đi qua query. Vé chỉ mở đúng một tệp và sống 60
   * giây — an toàn hơn nhiều so với việc đặt access token vào URL, vốn sẽ lọt
   * vào lịch sử trình duyệt và header Referer.
   */
  downloadUrl: async (path: string): Promise<string> => {
    const { ticket } = await request<{ ticket: string }>('/api/v1/files/ticket', {
      method: 'POST',
      body: { path },
    })
    return `${baseUrl()}/api/v1/files/download?ticket=${encodeURIComponent(ticket)}`
  },

  uploadUrl: () => `${baseUrl()}/api/v1/files/upload`,
}

/** Các endpoint quản lý dịch vụ hệ thống. */
export const serviceApi = {
  status: () => request<ServiceManagerStatus>('/api/v1/services/status'),

  list: () => request<SystemService[]>('/api/v1/services'),

  control: (name: string, action: ServiceAction) =>
    request<void>(`/api/v1/services/${encodeURIComponent(name)}/${action}`, { method: 'POST' }),

  logs: (name: string, lines = 200) =>
    request<{ logs: string }>(
      `/api/v1/services/${encodeURIComponent(name)}/logs?lines=${lines}`,
    ),
}

/** Các endpoint bảng tiến trình. */
export const processApi = {
  list: (keyword = '') =>
    request<ProcessList>(`/api/v1/processes?keyword=${encodeURIComponent(keyword)}`),

  listeners: () => request<PortListener[]>('/api/v1/processes/listeners'),

  kill: (pid: number, force = false) =>
    request<void>(`/api/v1/processes/${pid}?force=${force}`, { method: 'DELETE' }),
}

/** Các endpoint tác vụ định kỳ. */
export const cronApi = {
  list: () => request<CronJob[]>('/api/v1/cron'),

  create: (payload: CronJobPayload) =>
    request<CronJob>('/api/v1/cron', { method: 'POST', body: payload }),

  update: (id: number, payload: CronJobPayload) =>
    request<CronJob>(`/api/v1/cron/${id}`, { method: 'PUT', body: payload }),

  remove: (id: number) => request<void>(`/api/v1/cron/${id}`, { method: 'DELETE' }),

  setEnabled: (id: number, enabled: boolean) =>
    request<CronJob>(`/api/v1/cron/${id}/enabled`, { method: 'POST', body: { enabled } }),

  runNow: (id: number) => request<CronRun>(`/api/v1/cron/${id}/run`, { method: 'POST' }),

  runs: (id: number, limit = 50) => request<CronRun[]>(`/api/v1/cron/${id}/runs?limit=${limit}`),

  validate: (schedule: string) =>
    request<{ next: string[] }>('/api/v1/cron/validate', { method: 'POST', body: { schedule } }),
}

/** Các endpoint plugin. */
export const pluginApi = {
  list: () => request<PluginList>('/api/v1/plugins'),

  all: () => request<PluginList>('/api/v1/plugins/all'),

  reload: () => request<PluginList>('/api/v1/plugins/reload', { method: 'POST' }),

  /** Cấp vé ngắn hạn để khung nhúng mở được giao diện plugin. */
  ticket: (key: string) =>
    request<{ ticket: string }>(`/api/v1/plugins/${encodeURIComponent(key)}/ticket`, {
      method: 'POST',
    }),

  /** Địa chỉ đầy đủ để nhúng giao diện của một plugin.
   *
   * Khung nhúng không đặt được header Authorization, nên danh tính đi kèm dưới
   * dạng vé ngắn hạn chỉ mở được đúng plugin này. */
  uiUrl: (key: string, path: string, ticket: string) =>
    `${baseUrl()}/api/v1/plugins/${encodeURIComponent(key)}/proxy${path || '/'}` +
    `?ticket=${encodeURIComponent(ticket)}`,
}

/** Các endpoint quản lý node. */
export const nodeApi = {
  list: () => request<Node[]>('/api/v1/nodes'),

  get: (id: number) => request<Node>(`/api/v1/nodes/${id}`),

  create: (payload: NodePayload) =>
    request<Node>('/api/v1/nodes', { method: 'POST', body: payload }),

  update: (id: number, payload: NodePayload) =>
    request<Node>(`/api/v1/nodes/${id}`, { method: 'PUT', body: payload }),

  remove: (id: number) => request<void>(`/api/v1/nodes/${id}`, { method: 'DELETE' }),

  exec: (id: number, command: string) =>
    request<RemoteResult>(`/api/v1/nodes/${id}/exec`, { method: 'POST', body: { command } }),
}

/** Các endpoint cảnh báo. */
export const alertApi = {
  channels: () => request<NotifyChannel[]>('/api/v1/alerts/channels'),

  createChannel: (payload: NotifyChannelPayload) =>
    request<NotifyChannel>('/api/v1/alerts/channels', { method: 'POST', body: payload }),

  updateChannel: (id: number, payload: NotifyChannelPayload) =>
    request<NotifyChannel>(`/api/v1/alerts/channels/${id}`, { method: 'PUT', body: payload }),

  deleteChannel: (id: number) =>
    request<void>(`/api/v1/alerts/channels/${id}`, { method: 'DELETE' }),

  testChannel: (id: number) =>
    request<{ sent: boolean }>(`/api/v1/alerts/channels/${id}/test`, { method: 'POST' }),

  rules: () => request<AlertRule[]>('/api/v1/alerts/rules'),

  createRule: (payload: AlertRulePayload) =>
    request<AlertRule>('/api/v1/alerts/rules', { method: 'POST', body: payload }),

  updateRule: (id: number, payload: AlertRulePayload) =>
    request<AlertRule>(`/api/v1/alerts/rules/${id}`, { method: 'PUT', body: payload }),

  deleteRule: (id: number) => request<void>(`/api/v1/alerts/rules/${id}`, { method: 'DELETE' }),

  events: (limit = 50) => request<AlertEvent[]>(`/api/v1/alerts/events?limit=${limit}`),
}

/** Các endpoint khóa API. */
export const apiKeyApi = {
  list: () => request<ApiKey[]>('/api/v1/apikeys'),

  create: (payload: ApiKeyPayload) =>
    request<CreatedApiKey>('/api/v1/apikeys', { method: 'POST', body: payload }),

  setEnabled: (id: number, enabled: boolean) =>
    request<ApiKey>(`/api/v1/apikeys/${id}/enabled`, { method: 'POST', body: { enabled } }),

  remove: (id: number) => request<void>(`/api/v1/apikeys/${id}`, { method: 'DELETE' }),
}

/** Các endpoint sao lưu. */
export const backupApi = {
  list: () => request<BackupPlan[]>('/api/v1/backups'),

  create: (payload: BackupPlanPayload) =>
    request<BackupPlan>('/api/v1/backups', { method: 'POST', body: payload }),

  update: (id: number, payload: BackupPlanPayload) =>
    request<BackupPlan>(`/api/v1/backups/${id}`, { method: 'PUT', body: payload }),

  remove: (id: number) => request<void>(`/api/v1/backups/${id}`, { method: 'DELETE' }),

  setEnabled: (id: number, enabled: boolean) =>
    request<BackupPlan>(`/api/v1/backups/${id}/enabled`, { method: 'POST', body: { enabled } }),

  check: (payload: BackupPlanPayload) =>
    request<{ ok: boolean }>('/api/v1/backups/check', { method: 'POST', body: payload }),

  run: (id: number) => request<BackupRun>(`/api/v1/backups/${id}/run`, { method: 'POST' }),

  runs: (id: number, limit = 50) =>
    request<BackupRun[]>(`/api/v1/backups/${id}/runs?limit=${limit}`),

  objects: (id: number) => request<BackupObject[]>(`/api/v1/backups/${id}/objects`),

  deleteObject: (id: number, object: string) =>
    request<void>(`/api/v1/backups/${id}/objects/${encodeURIComponent(object)}`, {
      method: 'DELETE',
    }),

  restore: (id: number, object: string, target?: string) =>
    request<{ restored: string }>(`/api/v1/backups/${id}/restore`, {
      method: 'POST',
      body: { object, target: target ?? '' },
    }),
}

/** Các endpoint cơ sở dữ liệu. */
export const databaseApi = {
  servers: () => request<DatabaseServer[]>('/api/v1/db/servers'),

  createServer: (payload: DatabaseServerPayload) =>
    request<DatabaseServer>('/api/v1/db/servers', { method: 'POST', body: payload }),

  updateServer: (id: number, payload: DatabaseServerPayload) =>
    request<DatabaseServer>(`/api/v1/db/servers/${id}`, { method: 'PUT', body: payload }),

  deleteServer: (id: number) => request<void>(`/api/v1/db/servers/${id}`, { method: 'DELETE' }),

  databases: (id: number) => request<DatabaseInfo[]>(`/api/v1/db/servers/${id}/databases`),

  createDatabase: (id: number, name: string) =>
    request<{ name: string }>(`/api/v1/db/servers/${id}/databases`, {
      method: 'POST',
      body: { name },
    }),

  dropDatabase: (id: number, name: string) =>
    request<void>(`/api/v1/db/servers/${id}/databases/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    }),

  tables: (id: number, name: string) =>
    request<DatabaseTable[]>(`/api/v1/db/servers/${id}/databases/${encodeURIComponent(name)}/tables`),

  users: (id: number) => request<DatabaseUser[]>(`/api/v1/db/servers/${id}/users`),

  createUser: (id: number, payload: DatabaseUserPayload) =>
    request<{ name: string }>(`/api/v1/db/servers/${id}/users`, { method: 'POST', body: payload }),

  changePassword: (id: number, payload: DatabaseUserPayload) =>
    request<{ name: string }>(`/api/v1/db/servers/${id}/users/password`, {
      method: 'POST',
      body: payload,
    }),

  dropUser: (id: number, name: string, host?: string) =>
    request<void>(
      `/api/v1/db/servers/${id}/users/${encodeURIComponent(name)}` +
        (host ? `?host=${encodeURIComponent(host)}` : ''),
      { method: 'DELETE' },
    ),

  query: (id: number, database: string, statement: string) =>
    request<QueryResult>(`/api/v1/db/servers/${id}/query`, {
      method: 'POST',
      body: { database, statement },
    }),
}

/** Các endpoint chợ ứng dụng. */
export const appStoreApi = {
  status: () => request<ComposeStatus>('/api/v1/apps/status'),

  catalog: () => request<CatalogResult>('/api/v1/apps/catalog'),

  list: () => request<InstalledApp[]>('/api/v1/apps'),

  install: (payload: InstallPayload) =>
    request<InstalledApp>('/api/v1/apps', { method: 'POST', body: payload }),

  control: (id: number, action: AppAction) =>
    request<{ action: string }>(`/api/v1/apps/${id}/${action}`, { method: 'POST' }),

  logs: (id: number, lines = 200) =>
    request<{ logs: string }>(`/api/v1/apps/${id}/logs?lines=${lines}`),

  params: (id: number) => request<Record<string, string>>(`/api/v1/apps/${id}/params`),

  leftovers: () => request<AppLeftover[]>('/api/v1/apps/leftovers'),

  removeLeftover: (name: string) =>
    request<void>(`/api/v1/apps/leftovers/${encodeURIComponent(name)}`, { method: 'DELETE' }),

  uninstall: (id: number, removeData: boolean) =>
    request<void>(`/api/v1/apps/${id}?removeData=${removeData}`, { method: 'DELETE' }),
}

/** Các endpoint website. */
export const websiteApi = {
  status: () => request<WebServerStatus>('/api/v1/websites/status'),

  list: () => request<Website[]>('/api/v1/websites'),

  config: (id: number) => request<{ content: string }>(`/api/v1/websites/${id}/config`),

  rewrites: () => request<RewritePreset[]>('/api/v1/websites/rewrites'),

  domains: (id: number) => request<DomainReport>(`/api/v1/websites/${id}/domains`),

  traffic: (id: number, window: string) =>
    request<TrafficReport>(`/api/v1/websites/${id}/traffic?window=${window}`),

  create: (payload: WebsitePayload) =>
    request<Website>('/api/v1/websites', { method: 'POST', body: payload }),

  update: (id: number, payload: WebsitePayload) =>
    request<Website>(`/api/v1/websites/${id}`, { method: 'PUT', body: payload }),

  remove: (id: number) => request<void>(`/api/v1/websites/${id}`, { method: 'DELETE' }),

  setEnabled: (id: number, enabled: boolean) =>
    request<Website>(`/api/v1/websites/${id}/enabled`, { method: 'POST', body: { enabled } }),

  reload: () => request<{ reloaded: boolean }>('/api/v1/websites/reload', { method: 'POST' }),

  /** Triển khai mã nguồn: tải tệp nén lên, hoặc chỉ tới tệp đã có trên máy chủ. */
  deploySource: (
    id: number,
    payload: { file?: File; path?: string; clean: boolean; keepWrapper: boolean },
  ) => {
    const form = new FormData()
    if (payload.file) form.append('file', payload.file)
    if (payload.path) form.append('path', payload.path)
    form.append('clean', String(payload.clean))
    form.append('keepWrapper', String(payload.keepWrapper))

    return request<SourceDeployResult>(`/api/v1/websites/${id}/source`, {
      method: 'POST',
      body: form,
    })
  },
}

/** Các endpoint theo dõi uptime. */
export const uptimeApi = {
  list: () => request<UptimeMonitor[]>('/api/v1/uptime'),

  history: (id: number, limit = 60) =>
    request<UptimeCheck[]>(`/api/v1/uptime/${id}/history?limit=${limit}`),

  create: (payload: UptimeMonitorPayload) =>
    request<UptimeMonitor>('/api/v1/uptime', { method: 'POST', body: payload }),

  update: (id: number, payload: UptimeMonitorPayload) =>
    request<UptimeMonitor>(`/api/v1/uptime/${id}`, { method: 'PUT', body: payload }),

  remove: (id: number) => request<void>(`/api/v1/uptime/${id}`, { method: 'DELETE' }),

  check: (id: number) => request<UptimeMonitor>(`/api/v1/uptime/${id}/check`, { method: 'POST' }),
}

/** Các endpoint phân tích dung lượng ổ đĩa. */
export const diskApi = {
  partitions: () => request<DiskUsage[]>('/api/v1/disk/partitions'),

  usage: (path: string) =>
    request<DiskReport>(`/api/v1/disk/usage?path=${encodeURIComponent(path)}`),
}

/** Các endpoint xem nhật ký hệ thống. */
export const logApi = {
  sources: () => request<LogSource[]>('/api/v1/logs'),

  tail: (path: string, lines = 300) =>
    request<LogChunk>(`/api/v1/logs/content?path=${encodeURIComponent(path)}&lines=${lines}`),

  since: (path: string, offset: number) =>
    request<LogChunk>(`/api/v1/logs/content?path=${encodeURIComponent(path)}&offset=${offset}`),
}

/** Các endpoint tài khoản đăng nhập của máy chủ. */
export const systemAccountApi = {
  status: () => request<{ available: boolean }>('/api/v1/system-users/status'),

  list: () => request<SystemAccount[]>('/api/v1/system-users'),

  create: (payload: SystemAccountPayload) =>
    request<void>('/api/v1/system-users', { method: 'POST', body: payload }),

  setPassword: (name: string, password: string) =>
    request<void>(`/api/v1/system-users/${encodeURIComponent(name)}/password`, {
      method: 'POST',
      body: { password },
    }),

  setLocked: (name: string, locked: boolean) =>
    request<void>(`/api/v1/system-users/${encodeURIComponent(name)}/locked`, {
      method: 'POST',
      body: { locked },
    }),

  setSudo: (name: string, sudo: boolean) =>
    request<void>(`/api/v1/system-users/${encodeURIComponent(name)}/sudo`, {
      method: 'POST',
      body: { sudo },
    }),

  remove: (name: string, removeHome: boolean) =>
    request<void>(
      `/api/v1/system-users/${encodeURIComponent(name)}?removeHome=${removeHome}`,
      { method: 'DELETE' },
    ),

  keys: (name: string) => request<SshKey[]>(`/api/v1/system-users/${encodeURIComponent(name)}/keys`),

  addKey: (name: string, key: string) =>
    request<SshKey>(`/api/v1/system-users/${encodeURIComponent(name)}/keys`, {
      method: 'POST',
      body: { key },
    }),

  removeKey: (name: string, fingerprint: string) =>
    request<void>(
      `/api/v1/system-users/${encodeURIComponent(name)}/keys?fingerprint=${encodeURIComponent(fingerprint)}`,
      { method: 'DELETE' },
    ),
}

/** Các endpoint cấu hình của chính panel. */
export const settingsApi = {
  get: () => request<PanelSettingsInfo>('/api/v1/settings'),

  update: (payload: PanelSettings) =>
    request<PanelSettingsUpdate>('/api/v1/settings', { method: 'PUT', body: payload }),

  entryPath: () =>
    request<{ entryPath: string }>('/api/v1/settings/entry-path', { method: 'POST' }),

  restart: () => request<{ url: string }>('/api/v1/settings/restart', { method: 'POST' }),
}

/** Các endpoint chứng chỉ TLS. */
export const certApi = {
  list: () => request<Certificate[]>('/api/v1/certificates'),

  issue: (payload: CertPayload) =>
    request<Certificate>('/api/v1/certificates', { method: 'POST', body: payload }),

  renew: (name: string) =>
    request<Certificate>(`/api/v1/certificates/${encodeURIComponent(name)}/renew`, {
      method: 'POST',
    }),

  remove: (name: string) =>
    request<void>(`/api/v1/certificates/${encodeURIComponent(name)}`, { method: 'DELETE' }),
}

/** Các endpoint tường lửa. */
export const firewallApi = {
  status: () => request<FirewallStatus>('/api/v1/firewall/status'),

  rules: () => request<FirewallRule[]>('/api/v1/firewall/rules'),

  addRule: (payload: FirewallRulePayload) =>
    request<void>('/api/v1/firewall/rules', { method: 'POST', body: payload }),

  deleteRule: (id: string) =>
    request<void>(`/api/v1/firewall/rules/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  setEnabled: (enabled: boolean) =>
    request<void>('/api/v1/firewall/enabled', { method: 'POST', body: { enabled } }),
}

/** Các endpoint quản lý Docker. */
export const dockerApi = {
  status: () => request<DockerStatus>('/api/v1/docker/status'),

  containers: (all = true) =>
    request<DockerContainer[]>(`/api/v1/docker/containers?all=${all}`),

  control: (id: string, action: DockerAction) =>
    request<void>(`/api/v1/docker/containers/${encodeURIComponent(id)}/${action}`, {
      method: 'POST',
    }),

  logs: (id: string, lines = 200) =>
    request<{ logs: string }>(
      `/api/v1/docker/containers/${encodeURIComponent(id)}/logs?lines=${lines}`,
    ),

  stats: (id: string) =>
    request<DockerStats>(`/api/v1/docker/containers/${encodeURIComponent(id)}/stats`),

  images: () => request<DockerImage[]>('/api/v1/docker/images'),

  pullImage: (image: string) =>
    request<void>('/api/v1/docker/images/pull', { method: 'POST', body: { image } }),

  removeImage: (id: string, force = false) =>
    request<void>(`/api/v1/docker/images/${encodeURIComponent(id)}?force=${force}`, {
      method: 'DELETE',
    }),

  volumes: () => request<DockerVolume[]>('/api/v1/docker/volumes'),

  removeVolume: (name: string) =>
    request<void>(`/api/v1/docker/volumes/${encodeURIComponent(name)}`, { method: 'DELETE' }),

  networks: () => request<DockerNetwork[]>('/api/v1/docker/networks'),

  prune: () => request<{ freed: number }>('/api/v1/docker/prune', { method: 'POST' }),
}

/** Endpoint rà soát tình trạng máy chủ. */
export const healthApi = {
  report: () => request<HealthReport>('/api/v1/health/report'),
}

/** Các endpoint phòng thủ đăng nhập. */
export const securityApi = {
  overview: () => request<SecurityOverview>('/api/v1/security'),

  unblock: (ip: string) =>
    request<void>(`/api/v1/security/blocks/${encodeURIComponent(ip)}`, { method: 'DELETE' }),
}

/** Các endpoint nhật ký kiểm toán. */
export const auditApi = {
  list: (page = 1, pageSize = 20) =>
    request<PageResult<AuditLog>>(`/api/v1/audit?page=${page}&pageSize=${pageSize}`),

  logins: (page = 1, pageSize = 20) =>
    request<PageResult<LoginLog>>(`/api/v1/audit/logins?page=${page}&pageSize=${pageSize}`),
}

export * from './types'
export { ApiError, wsUrl } from './client'
