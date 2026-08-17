/** Khung phản hồi chung của mọi endpoint. */
export interface ApiResponse<T> {
  success: boolean
  data?: T
  /** Mã lỗi để tra bảng dịch, ví dụ "auth.invalid_credentials". */
  code?: string
  /** Tham số chèn vào chuỗi dịch. */
  params?: Record<string, unknown>
  requestId?: string
}

export type Role = 'admin' | 'operator' | 'readonly'

export interface User {
  id: number
  username: string
  email: string
  role: Role
  language: string
  theme: string
  totpEnabled: boolean
  active: boolean
  mustChangePassword: boolean
  lastLoginAt?: string
  lastLoginIP?: string
  createdAt: string
  updatedAt: string
}

export interface TokenPair {
  accessToken: string
  refreshToken: string
  expiresAt: string
  user: User
}

export interface Session {
  id: number
  userId: number
  userAgent: string
  ip: string
  expiresAt: string
  createdAt: string
  lastUsed: string
}

export interface DiskUsage {
  mountpoint: string
  device: string
  fstype: string
  total: number
  used: number
  free: number
  percent: number
}

/** Ảnh chụp tài nguyên tại một thời điểm. */
export interface Snapshot {
  time: string
  cpu: number
  cpuPerCore: number[] | null
  load1: number
  load5: number
  load15: number
  memoryTotal: number
  memoryUsed: number
  memory: number
  swapTotal: number
  swapUsed: number
  swap: number
  disks: DiskUsage[] | null
  disk: number
  netSent: number
  netRecv: number
  diskRead: number
  diskWrite: number
  uptime: number
}

export interface SystemInfo {
  hostname: string
  os: string
  platform: string
  version: string
  kernel: string
  arch: string
  cpuModel: string
  cpuCores: number
  totalMemory: number
  bootTime: number
  virtualization: string
}

export interface Overview {
  system: SystemInfo
  current: Snapshot
}

/** Một mẫu lịch sử dùng để vẽ biểu đồ. */
export interface HistorySample {
  time: string
  cpu: number
  memory: number
  swap: number
  disk: number
  load1: number
  netSent: number
  netRecv: number
  diskRead: number
  diskWrite: number
}

export interface TotpSetup {
  secret: string
  url: string
}

export interface AuditLog {
  id: number
  userId: number
  username: string
  action: string
  resource: string
  ip: string
  success: boolean
  detail?: string
  createdAt: string
}

export interface LoginLog {
  id: number
  username: string
  ip: string
  userAgent: string
  success: boolean
  reason?: string
  createdAt: string
}

export interface PageResult<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}
