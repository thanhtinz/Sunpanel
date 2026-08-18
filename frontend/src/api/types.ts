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

export interface FileInfo {
  name: string
  path: string
  size: number
  mode: string
  isDir: boolean
  isLink: boolean
  linkTarget?: string
  modTime: string
  owner?: string
  group?: string
}

export interface FileList {
  path: string
  parent: string
  items: FileInfo[]
  total: number
}

export interface FileContent {
  path: string
  content: string
  size: number
  mode: string
}

export type ArchiveFormat = 'zip' | 'tar.gz'

export type ServiceState =
  | 'active'
  | 'inactive'
  | 'failed'
  | 'activating'
  | 'deactivating'
  | 'unknown'

export interface SystemService {
  name: string
  description: string
  state: ServiceState
  subState: string
  enabled: boolean
  enabledState: string
  loaded: boolean
  protected: boolean
}

export interface ServiceManagerStatus {
  available: boolean
  manager: string
}

export type ServiceAction = 'start' | 'stop' | 'restart' | 'reload' | 'enable' | 'disable'

export interface CronJob {
  id: number
  name: string
  schedule: string
  command: string
  workDir: string
  timeoutSeconds: number
  enabled: boolean
  lastRunAt?: string
  lastSuccess?: boolean
  lastExitCode: number
  nextRunAt?: string
  createdAt: string
  updatedAt: string
}

export interface CronRun {
  id: number
  jobId: number
  jobName: string
  startedAt: string
  finishedAt?: string
  durationMs: number
  exitCode: number
  success: boolean
  output: string
  error?: string
  triggeredBy: string
}

export interface CronJobPayload {
  name: string
  schedule: string
  command: string
  workDir?: string
  timeoutSeconds?: number
  enabled?: boolean
}

export type FirewallAction = 'allow' | 'deny' | 'reject'
export type FirewallProtocol = 'tcp' | 'udp' | 'any'

export interface FirewallRule {
  id: string
  action: FirewallAction
  protocol: FirewallProtocol
  port: string
  source: string
  description: string
  ipv6: boolean
  raw?: string
}

export interface FirewallStatus {
  backend: string
  available: boolean
  enabled: boolean
}

export interface FirewallRulePayload {
  action: FirewallAction
  protocol: FirewallProtocol
  port?: string
  source?: string
  description?: string
}

export type ContainerState =
  | 'running'
  | 'exited'
  | 'paused'
  | 'created'
  | 'restarting'
  | 'dead'

export interface PortMapping {
  publicPort: number
  privatePort: number
  type: string
  ip?: string
}

export interface DockerContainer {
  id: string
  name: string
  image: string
  state: ContainerState
  status: string
  created: number
  ports: PortMapping[] | null
  compose?: string
  protected: boolean
}

export interface DockerImage {
  id: string
  tags: string[] | null
  size: number
  created: number
  dangling: boolean
}

export interface DockerVolume {
  name: string
  driver: string
  mountpoint: string
  createdAt: string
}

export interface DockerNetwork {
  id: string
  name: string
  driver: string
  scope: string
  internal: boolean
  subnets: string[] | null
}

export interface DockerInfo {
  version: string
  apiVersion: string
  containers: number
  containersRunning: number
  images: number
  storageDriver: string
  diskUsed: number
  reclaimable: number
}

export interface DockerStatus {
  available: boolean
  socket: string
  info: DockerInfo
}

export interface DockerStats {
  cpuPercent: number
  memoryUsage: number
  memoryLimit: number
  memoryPercent: number
  networkRx: number
  networkTx: number
}

export type DockerAction = 'start' | 'stop' | 'restart' | 'pause' | 'unpause' | 'remove'

export type WebsiteType = 'static' | 'proxy' | 'php'

export interface Website {
  id: number
  name: string
  /** Danh sách tên miền, ngăn cách bằng dấu phẩy. */
  domains: string
  type: WebsiteType
  root: string
  proxyTarget: string
  phpSocket: string
  port: number
  sslPort: number
  sslEnabled: boolean
  forceHttps: boolean
  certName: string
  extraConfig: string
  enabled: boolean
  remark: string
  createdAt: string
  updatedAt: string
}

export interface WebsitePayload {
  name: string
  domains: string[]
  type: WebsiteType
  root?: string
  proxyTarget?: string
  phpSocket?: string
  port?: number
  sslPort?: number
  sslEnabled?: boolean
  forceHttps?: boolean
  certName?: string
  extraConfig?: string
  enabled?: boolean
  remark?: string
}

export interface WebServerStatus {
  server: string
  available: boolean
}

export type CertSource = 'self' | 'acme' | 'upload'

export interface Certificate {
  id: number
  name: string
  domains: string
  source: CertSource
  email: string
  staging: boolean
  autoRenew: boolean
  issuedAt: string
  expiresAt: string
  lastRenewAt?: string
  lastError?: string
  issuer: string
  daysRemaining: number
  /** Bản ghi còn trong cơ sở dữ liệu nhưng tệp chứng chỉ đã biến mất. */
  missing: boolean
  createdAt: string
  updatedAt: string
}

export interface CertPayload {
  name: string
  domains: string[]
  source: CertSource
  email?: string
  staging?: boolean
  autoRenew?: boolean
  certPem?: string
  keyPem?: string
}

export interface LocalizedText {
  vi: string
  en: string
}

export type AppFieldType = 'text' | 'password' | 'port' | 'number' | 'select'

export interface AppFieldOption {
  value: string
  label: LocalizedText
}

export interface AppField {
  key: string
  label: LocalizedText
  help: LocalizedText
  type: AppFieldType
  default: string
  required: boolean
  generate: boolean
  min: number
  max: number
  options: AppFieldOption[] | null
}

export interface CatalogApp {
  key: string
  name: LocalizedText
  description: LocalizedText
  category: string
  version: string
  website: string
  images: string[] | null
  portField: string
  fields: AppField[] | null
}

export interface CatalogResult {
  apps: CatalogApp[]
  categories: string[] | null
}

export interface InstalledApp {
  id: number
  name: string
  appKey: string
  version: string
  dir: string
  containerName: string
  port: number
  remark: string
  lastError?: string
  createdAt: string
  updatedAt: string
  app?: CatalogApp
  running: boolean
  state: string
}

export interface InstallPayload {
  appKey: string
  name: string
  values: Record<string, string>
  remark?: string
}

export interface ComposeStatus {
  available: boolean
  version: string
}

export type AppAction = 'start' | 'stop' | 'restart'
