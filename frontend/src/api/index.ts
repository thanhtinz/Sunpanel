import { request } from './client'
import type {
  AuditLog,
  HistorySample,
  LoginLog,
  Overview,
  PageResult,
  Role,
  Session,
  Snapshot,
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

/** Các endpoint nhật ký kiểm toán. */
export const auditApi = {
  list: (page = 1, pageSize = 20) =>
    request<PageResult<AuditLog>>(`/api/v1/audit?page=${page}&pageSize=${pageSize}`),

  logins: (page = 1, pageSize = 20) =>
    request<PageResult<LoginLog>>(`/api/v1/audit/logins?page=${page}&pageSize=${pageSize}`),
}

export * from './types'
export { ApiError, wsUrl } from './client'
