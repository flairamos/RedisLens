// 后端绑定封装（wailsjs 生成代码之上再包一层错误处理）
import * as wailsApp from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import type {
  AuditEntry, BigKeyReport, CmdStat, CommandHelp, ConnDTO, ConsoleResult,
  DeletePreview, DeleteResult, ExportReport, FieldItem, ImportReport, KeyItem,
  RecycleItem, Sample, ScanPageResp, ServerSummary, Settings, SlowLogEntry,
  TreeNode, TTLResult, Topology, ValueData,
} from './types'

const A = wailsApp as any

export const api = {
  // 基础
  getAppInfo: () => A.GetAppInfo() as Promise<Record<string, any>>,
  getSettings: () => A.GetSettings() as Promise<Settings>,
  updateSettings: (s: Settings) => A.UpdateSettings(s) as Promise<void>,
  queryAudit: (limit: number) => A.QueryAudit(limit) as Promise<AuditEntry[]>,

  // 连接
  listConnections: () => A.ListConnections() as Promise<ConnDTO[]>,
  saveConnection: (c: any, clearPassword: boolean) => A.SaveConnection(c, clearPassword) as Promise<ConnDTO>,
  deleteConnection: (id: string) => A.DeleteConnection(id) as Promise<void>,
  testConnection: (c: any) => A.TestConnectionConfig(c) as Promise<ServerSummary>,
  openConnection: (id: string) => A.OpenConnection(id) as Promise<ServerSummary>,
  closeConnection: (id: string) => A.CloseConnection(id) as Promise<void>,
  getTopology: (id: string) => A.GetTopology(id) as Promise<Topology>,

  // 键空间
  scanPage: (conn: string, db: number, cursor: number, match: string, typeFilter: string, count: number) =>
    A.ScanPage(conn, db, cursor, match, typeFilter, count) as Promise<ScanPageResp>,
  keyTree: (conn: string, db: number, prefix: string) => A.KeyTree(conn, db, prefix) as Promise<TreeNode[]>,
  getValue: (conn: string, db: number, key: string) => A.GetValue(conn, db, key) as Promise<ValueData>,
  scanStreamPage: (conn: string, db: number, key: string, typ: string, cursor: number, match: string, count: number) =>
    A.ScanStreamPage(conn, db, key, typ, cursor, match, count) as Promise<[number, number, FieldItem[], Error]>,
  createKey: (conn: string, db: number, key: string, typ: string, initial: string, ttlMs: number) =>
    A.CreateKey(conn, db, key, typ, initial, ttlMs) as Promise<void>,
  setString: (conn: string, db: number, key: string, value: string, binary: boolean, ttlMs: number) =>
    A.SetString(conn, db, key, value, binary, ttlMs) as Promise<void>,
  hashSetField: (conn: string, db: number, key: string, field: string, value: string, binary: boolean) =>
    A.HashSetField(conn, db, key, field, value, binary) as Promise<void>,
  hashDelFields: (conn: string, db: number, key: string, fields: string[]) =>
    A.HashDelFields(conn, db, key, fields) as Promise<void>,
  listSetIndex: (conn: string, db: number, key: string, index: number, value: string, binary: boolean) =>
    A.ListSetIndex(conn, db, key, index, value, binary) as Promise<void>,
  listPush: (conn: string, db: number, key: string, side: string, value: string, binary: boolean) =>
    A.ListPush(conn, db, key, side, value, binary) as Promise<void>,
  listRemoveIndex: (conn: string, db: number, key: string, index: number) =>
    A.ListRemoveIndex(conn, db, key, index) as Promise<void>,
  setAddMembers: (conn: string, db: number, key: string, members: string[]) =>
    A.SetAddMembers(conn, db, key, members) as Promise<void>,
  setRemoveMembers: (conn: string, db: number, key: string, members: string[]) =>
    A.SetRemoveMembers(conn, db, key, members) as Promise<void>,
  zsetUpsert: (conn: string, db: number, key: string, member: string, score: number, binary: boolean) =>
    A.ZSetUpsert(conn, db, key, member, score, binary) as Promise<void>,
  zsetRemoveMembers: (conn: string, db: number, key: string, members: string[]) =>
    A.ZSetRemoveMembers(conn, db, key, members) as Promise<void>,
  streamAdd: (conn: string, db: number, key: string, id: string, fields: Record<string, string>) =>
    A.StreamAdd(conn, db, key, id, fields) as Promise<void>,
  streamDelete: (conn: string, db: number, key: string, ids: string[]) =>
    A.StreamDelete(conn, db, key, ids) as Promise<void>,
  jsonSet: (conn: string, db: number, key: string, doc: string) =>
    A.JSONSet(conn, db, key, doc) as Promise<void>,
  deleteKeys: (conn: string, db: number, keys: string[], backup: boolean) =>
    A.DeleteKeys(conn, db, keys, backup) as Promise<DeleteResult>,
  renameKey: (conn: string, db: number, oldKey: string, newKey: string) =>
    A.RenameKey(conn, db, oldKey, newKey) as Promise<void>,
  setTTL: (conn: string, db: number, keys: string[], ttlMs: number) =>
    A.SetTTL(conn, db, keys, ttlMs) as Promise<TTLResult>,
  recycleList: (conn: string) => A.RecycleList(conn) as Promise<RecycleItem[]>,
  recycleRestore: (conn: string, token: string) => A.RecycleRestore(conn, token) as Promise<void>,
  recycleRemove: (conn: string, token: string) => A.RecycleRemove(conn, token) as Promise<void>,
  recycleClear: (conn: string) => A.RecycleClear(conn) as Promise<void>,

  // 监控
  monitorHistory: (conn: string, seconds: number) => A.MonitorHistory(conn, seconds) as Promise<Sample[]>,

  // 控制台
  execCommand: (conn: string, line: string, confirm: boolean) => A.ExecCommand(conn, line, confirm) as Promise<ConsoleResult>,
  completeKeys: (conn: string, db: number, prefix: string) => A.CompleteKeys(conn, db, prefix) as Promise<string[]>,
  commandHelpList: () => A.CommandHelpList() as Promise<CommandHelp[]>,

  // 分析
  beginBigKeyScan: (conn: string, topN: number, maxScan: number) => A.BeginBigKeyScan(conn, topN, maxScan) as Promise<string>,
  stopBigKeyScan: (task: string) => A.StopBigKeyScan(task) as Promise<void>,
  getSlowLog: (conn: string, limit: number) => A.GetSlowLog(conn, limit) as Promise<SlowLogEntry[]>,
  getCommandStats: (conn: string) => A.GetCommandStats(conn) as Promise<CmdStat[]>,

  // 导入导出
  previewDelete: (conn: string, db: number, match: string, typeFilter: string, maxPreview: number) =>
    A.PreviewDelete(conn, db, match, typeFilter, maxPreview) as Promise<DeletePreview>,
  exportKeys: (conn: string, db: number, match: string, typeFilter: string, file: string) =>
    A.ExportKeys(conn, db, match, typeFilter, file) as Promise<ExportReport>,
  exportCSV: (conn: string, db: number, match: string, typeFilter: string, file: string) =>
    A.ExportCSV(conn, db, match, typeFilter, file) as Promise<ExportReport>,
  importFile: (conn: string, db: number, file: string, overwrite: boolean) =>
    A.ImportFile(conn, db, file, overwrite) as Promise<ImportReport>,

  // 发布订阅
  pubsubSubscribe: (conn: string, channels: string[], patterns: string[]) =>
    A.PubSubSubscribe(conn, channels, patterns) as Promise<void>,
  pubsubUnsubscribe: (conn: string) => A.PubSubUnsubscribe(conn) as Promise<void>,
  publish: (conn: string, channel: string, payload: string) => A.Publish(conn, channel, payload) as Promise<number>,

  // 对话框
  pickSaveFile: (title: string, defaultName: string) => A.PickSaveFile(title, defaultName) as Promise<string>,
  pickOpenFile: (title: string) => A.PickOpenFile(title) as Promise<string>,
  pickFile: (title: string) => A.PickFile(title) as Promise<string>,
}

// Wails 事件订阅（组件卸载时需调用 off）
export function onEvent(name: string, cb: (...data: any[]) => void) {
  EventsOn(name, cb)
  return () => EventsOff(name)
}

// 统一错误消息提取
export function errMsg(e: any): string {
  if (!e) return '未知错误'
  if (typeof e === 'string') return e
  return e.message || String(e)
}
