// 类型定义（与后端 Go 结构对应）
export interface SSHConfig {
  enabled: boolean
  host: string
  port: number
  user: string
  authType: 'password' | 'key'
  password?: string
  keyPath?: string
  passphrase?: string
}

export interface TLSConfig {
  enabled: boolean
  certFile?: string
  keyFile?: string
  caFile?: string
  serverName?: string
  insecureSkipVerify?: boolean
}

export interface SentinelConfig {
  master: string
  addrs: string[]
  username?: string
  password?: string
}

export interface Connection {
  id: string
  name: string
  group: string
  mode: 'standalone' | 'sentinel' | 'cluster'
  host: string
  port: number
  username?: string
  password?: string
  db: number
  readOnly: boolean
  ssh: SSHConfig
  tls: TLSConfig
  sentinel: SentinelConfig
  createdAt?: string
  lastUsedAt?: string
}

export interface ConnDTO extends Connection {
  hasPassword: boolean
  hasSSHPass: boolean
  hasSSHKey: boolean
  health: 'ok' | 'down' | 'connecting' | 'closed'
  healthError: string
  open: boolean
}

export interface ServerSummary {
  version: string
  mode: string
  role: string
  os: string
  usedMem: number
  totalKeys: number
}

export interface Settings {
  theme: 'light' | 'dark'
  density: 'compact' | 'comfortable' | 'spacious'
  monitorIntervalSec: number
  confirmDanger: boolean
  logEnabled: boolean
  scanCount: number
  valuePreviewLimit: number
}

export interface KeyItem {
  key: string
  type: string
  ttl: number
  size: number
}

export interface ScanPageResp {
  cursor: number
  keys: KeyItem[]
  done: boolean
  scanned: number
}

export interface TreeNode {
  name: string
  prefix: string
  isFolder: boolean
  type?: string
  ttl?: number
}

export interface FieldItem {
  field: string
  value: string
  binary: boolean
}

export interface ListItem {
  index: number
  value: string
  binary: boolean
}

export interface ZSetItem {
  rank: number
  member: string
  score: number
}

export interface StreamEntry {
  id: string
  fields: Record<string, string>
}

export interface ValueData {
  key: string
  type: string
  encoding?: string
  ttl: number
  size: number
  elements: number
  binary: boolean
  str?: string
  isJson?: boolean
  hash?: FieldItem[]
  list?: ListItem[]
  set?: string[]
  zset?: ZSetItem[]
  stream?: StreamEntry[]
}

export interface DeleteResult {
  deleted: number
  backedUp: number
  tokens: string[]
}

export interface TTLResult {
  affected: number
}

export interface RecycleItem {
  token: string
  key: string
  type: string
  pttl: number
  deletedAt: string
  size: number
}

export interface Sample {
  time: number
  qps: number
  hitRate: number
  hitsRate: number
  missRate: number
  usedMem: number
  maxMem: number
  rssMem: number
  fragRatio: number
  clients: number
  blocked: number
  netInRate: number
  netOutRate: number
  totalKeys: number
  expiredRate: number
  evictedRate: number
  cpuUserRate: number
  cpuSysRate: number
  role: string
  slaves: number
  replLag: number
  masterLink: string
  version: string
}

export interface ConsoleResult {
  ok: boolean
  blocked: boolean
  needConfirm: boolean
  reason?: string
  op?: string
  reply: string
  rows?: string[]
  duration: number
}

export interface CommandHelp {
  name: string
  group: string
  synop: string
  danger: boolean
}

export interface BigKeyItem {
  key: string
  type: string
  elements: number
  bytes: number
}

export interface BigKeyReport {
  scannedKeys: number
  totalKeys: number
  durationMs: number
  topByMemory: BigKeyItem[]
  topByElement: BigKeyItem[]
  typeDist: Record<string, number>
  ttlBuckets: Record<string, number>
  stopped: boolean
}

export interface SlowLogEntry {
  id: number
  time: number
  duration: number
  command: string
  client: string
  clientName: string
}

export interface CmdStat {
  command: string
  calls: number
  usec: number
}

export interface DeletePreview {
  match: string
  total: number
  samples: KeyItem[]
}

export interface ExportReport {
  file: string
  keys: number
  startedAt: string
  finishedAt: string
  durationMs: number
}

export interface ImportReport {
  file: string
  total: number
  imported: number
  skipped: number
  failed: number
  durationMs: number
}

export interface NodeInfo {
  id?: string
  addr: string
  role: string
  slots?: string
  state?: string
}

export interface Topology {
  mode: string
  master?: string
  nodes: NodeInfo[]
  shards?: number
}

export interface AuditEntry {
  time: string
  connId: string
  connName: string
  category: string
  action: string
  target: string
  detail?: string
  result: string
  level: string
}

export interface PubSubMessage {
  connId: string
  kind: string
  channel: string
  pattern?: string
  payload: string
  binary: boolean
  ts: number
}
