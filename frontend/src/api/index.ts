import { baseUrl, request } from './client'
import type {
  ArchiveFormat,
  AuditLog,
  FileContent,
  FileInfo,
  FileList,
  HistorySample,
  LoginLog,
  Overview,
  PageResult,
  Role,
  ServiceAction,
  ServiceManagerStatus,
  Session,
  Snapshot,
  SystemService,
  TokenPair,
  TotpSetup,
  User,
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
    request<void>('/api/v1/files/extract', { method: 'POST', body: { path, target } }),

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

/** Các endpoint nhật ký kiểm toán. */
export const auditApi = {
  list: (page = 1, pageSize = 20) =>
    request<PageResult<AuditLog>>(`/api/v1/audit?page=${page}&pageSize=${pageSize}`),

  logins: (page = 1, pageSize = 20) =>
    request<PageResult<LoginLog>>(`/api/v1/audit/logins?page=${page}&pageSize=${pageSize}`),
}

export * from './types'
export { ApiError, wsUrl } from './client'
