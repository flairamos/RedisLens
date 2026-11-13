package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"RedisLens/internal/audit"
	"RedisLens/internal/config"
	"RedisLens/internal/rds"
)

// Version 应用版本号。
const Version = "1.0.0"

// App Wails 绑定层：前端可调用的全部方法在此聚合，业务逻辑下沉到 internal 包。
type App struct {
	ctx     context.Context
	store   *config.Store
	audit   *audit.Logger
	mgr     *rds.Manager
	logFile *os.File
	startAt time.Time
}

// NewApp 构造应用（配置在 startup 时加载，避免构造期 IO）。
func NewApp() *App {
	return &App{}
}

// startup Wails 生命周期：初始化存储、审计、管理器。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.startAt = time.Now()
	cfgDir, err := appConfigDir()
	if err != nil {
		log.Fatalf("resolve config dir: %v", err)
	}
	store, err := config.Open(cfgDir)
	if err != nil {
		log.Fatalf("open config: %v", err)
	}
	a.store = store

	// 日志开关
	if store.Settings().LogEnabled {
		a.openLogFile(cfgDir)
	}

	// 审计日志（月度轮转 + 90 天清理）
	auditDir := filepath.Join(cfgDir, "audit")
	_ = os.MkdirAll(auditDir, 0o700)
	a.audit = audit.New(auditDir, true)
	a.audit.Cleanup()

	// 连接管理器：事件转发到前端
	a.mgr = rds.NewManager(store, a.audit, func(event string, payload any) {
		runtime.EventsEmit(ctx, event, payload)
	})
}

// shutdown 生命周期收尾：关闭全部连接与文件句柄。
func (a *App) shutdown(ctx context.Context) {
	a.mgr.CloseAll()
	a.audit.Close()
	if a.logFile != nil {
		_ = a.logFile.Close()
	}
}

// ---------- 基础 ----------

// GetAppInfo 返回应用信息。
func (a *App) GetAppInfo() map[string]any {
	return map[string]any{
		"version":  Version,
		"dataDir":  a.store.Dir(),
		"startedAt": a.startAt.Format("2006-01-02 15:04:05"),
	}
}

// GetSettings 读取设置。
func (a *App) GetSettings() config.Settings { return a.store.Settings() }

// UpdateSettings 保存设置（含主题、密度、采样间隔等）。
func (a *App) UpdateSettings(s config.Settings) error {
	old := a.store.Settings()
	if err := a.store.UpdateSettings(s); err != nil {
		return err
	}
	if s.LogEnabled != old.LogEnabled {
		if s.LogEnabled {
			a.openLogFile(a.store.Dir())
		} else {
			a.closeLogFile()
		}
	}
	// 采样间隔热更新
	for _, id := range a.mgr.ListOpen() {
		if b, err := a.mgr.Get(id); err == nil && b.Monitor != nil {
			b.Monitor.SetInterval(time.Duration(s.MonitorIntervalSec) * time.Second)
		}
	}
	return nil
}

// QueryAudit 读取审计日志。
func (a *App) QueryAudit(limit int) ([]audit.Entry, error) {
	return a.audit.Query(limit)
}

// ---------- 连接管理 ----------

// connDTO 前端可见的连接结构（不回传机密字段）。
type connDTO struct {
	*config.Connection
	HasPassword bool   `json:"hasPassword"`
	HasSSHPass  bool   `json:"hasSSHPass"`
	HasSSHKey   bool   `json:"hasSSHKey"`
	Health      string `json:"health"`
	HealthError string `json:"healthError"`
	Open        bool   `json:"open"`
}

func (a *App) toDTO(c *config.Connection) *connDTO {
	d := &connDTO{
		Connection:  c,
		HasPassword: c.Password != "",
		HasSSHPass:  c.SSHTunnel.Password != "",
		HasSSHKey:   c.SSHTunnel.KeyPath != "",
		Open:        a.mgr.IsOpen(c.ID),
	}
	d.Password = ""
	d.SentinelPass = ""
	d.SSHTunnel.Password = ""
	d.SSHTunnel.Passphrase = ""
	if a.mgr.IsOpen(c.ID) {
		if b, err := a.mgr.Get(c.ID); err == nil {
			h, e := b.Health()
			d.Health = string(h)
			d.HealthError = e
		}
	} else {
		d.Health = "closed"
	}
	return d
}

// ListConnections 连接列表（机密字段脱敏）。
func (a *App) ListConnections() []*connDTO {
	list := a.store.ListConnections()
	out := make([]*connDTO, 0, len(list))
	for _, c := range list {
		out = append(out, a.toDTO(c))
	}
	return out
}

// SaveConnection 新增/更新连接。机密字段留空表示沿用原值。
func (a *App) SaveConnection(c *config.Connection, clearPassword bool) (*connDTO, error) {
	if c.Name == "" {
		return nil, fmt.Errorf("连接名称不能为空")
	}
	if c.Port == 0 {
		c.Port = 6379
	}
	if c.Mode == "" {
		c.Mode = "standalone"
	}
	if c.Mode == "cluster" {
		c.DB = 0
	}
	if old, ok := a.store.GetConnection(c.ID); ok {
		if c.Password == "" && !clearPassword {
			c.Password = old.Password
		}
		if c.SentinelPass == "" {
			c.SentinelPass = old.SentinelPass
		}
		if c.SSHTunnel.Password == "" {
			c.SSHTunnel.Password = old.SSHTunnel.Password
		}
		if c.SSHTunnel.Passphrase == "" {
			c.SSHTunnel.Passphrase = old.SSHTunnel.Passphrase
		}
		if c.CreatedAt.IsZero() {
			c.CreatedAt = old.CreatedAt
		}
	}
	if err := a.store.UpsertConnection(c); err != nil {
		return nil, err
	}
	return a.toDTO(c), nil
}

// DeleteConnection 删除连接（若已打开则先关闭）。
func (a *App) DeleteConnection(id string) error {
	a.mgr.Close(id)
	return a.store.DeleteConnection(id)
}

// TestConnectionConfig 测试连接配置（未保存亦可测试）。
func (a *App) TestConnectionConfig(c *config.Connection) (*rds.ServerSummary, error) {
	if old, ok := a.store.GetConnection(c.ID); ok {
		if c.Password == "" {
			c.Password = old.Password
		}
		if c.SSHTunnel.Password == "" {
			c.SSHTunnel.Password = old.SSHTunnel.Password
		}
		if c.SSHTunnel.Passphrase == "" {
			c.SSHTunnel.Passphrase = old.SSHTunnel.Passphrase
		}
		if c.SentinelPass == "" {
			c.SentinelPass = old.SentinelPass
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return rds.TestConnection(ctx, c)
}

// OpenConnection 打开连接（含心跳与监控启动）。
func (a *App) OpenConnection(id string) (*rds.ServerSummary, error) {
	c, ok := a.store.GetConnection(id)
	if !ok {
		return nil, fmt.Errorf("connection %s not found", id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return a.mgr.Open(ctx, c)
}

// CloseConnection 关闭连接。
func (a *App) CloseConnection(id string) error {
	a.mgr.Close(id)
	return nil
}

// GetTopology 拓扑发现。
func (a *App) GetTopology(connID string) (*rds.Topology, error) {
	return a.mgr.GetTopology(connID)
}

// ---------- 键空间 ----------

// ScanPage SCAN 分页。
func (a *App) ScanPage(connID string, db int, cursor uint64, match, typeFilter string, count int) (*rds.ScanPageResp, error) {
	return a.mgr.ScanPage(connID, db, cursor, match, typeFilter, count)
}

// KeyTree 键空间树。
func (a *App) KeyTree(connID string, db int, prefix string) ([]rds.TreeNode, error) {
	return a.mgr.KeyTree(connID, db, prefix)
}

// GetValue 键值查看。
func (a *App) GetValue(connID string, db int, key string) (*rds.ValueData, error) {
	return a.mgr.GetValue(connID, db, key)
}

// ScanStreamPage 集合类型游标分页（hash/set）。
func (a *App) ScanStreamPage(connID string, db int, key, typ string, cursor uint64, match string, count int64) (uint64, int64, []rds.FieldItem, error) {
	return a.mgr.ScanStreamPage(connID, db, key, typ, cursor, match, count)
}

// CreateKey 新建键。
func (a *App) CreateKey(connID string, db int, key, typ, initial string, ttlMs int64) error {
	return a.mgr.CreateKey(connID, db, key, typ, initial, ttlMs)
}

// SetString 字符串编辑。
func (a *App) SetString(connID string, db int, key, value string, binary bool, ttlMs int64) error {
	return a.mgr.SetString(connID, db, key, value, binary, ttlMs)
}

// HashSetField 哈希字段写入。
func (a *App) HashSetField(connID string, db int, key, field, value string, binary bool) error {
	return a.mgr.HashSetField(connID, db, key, field, value, binary)
}

// HashDelFields 哈希字段删除。
func (a *App) HashDelFields(connID string, db int, key string, fields []string) error {
	return a.mgr.HashDelFields(connID, db, key, fields)
}

// ListSetIndex 列表元素修改。
func (a *App) ListSetIndex(connID string, db int, key string, index int64, value string, binary bool) error {
	return a.mgr.ListSetIndex(connID, db, key, index, value, binary)
}

// ListPush 列表推入。
func (a *App) ListPush(connID string, db int, key, side, value string, binary bool) error {
	return a.mgr.ListPush(connID, db, key, side, value, binary)
}

// ListRemoveIndex 列表元素删除。
func (a *App) ListRemoveIndex(connID string, db int, key string, index int64) error {
	return a.mgr.ListRemoveIndex(connID, db, key, index)
}

// SetAddMembers 集合添加。
func (a *App) SetAddMembers(connID string, db int, key string, members []string) error {
	return a.mgr.SetAddMembers(connID, db, key, members)
}

// SetRemoveMembers 集合删除。
func (a *App) SetRemoveMembers(connID string, db int, key string, members []string) error {
	return a.mgr.SetRemoveMembers(connID, db, key, members)
}

// ZSetUpsert 有序集合写入。
func (a *App) ZSetUpsert(connID string, db int, key, member string, score float64, memberBinary bool) error {
	return a.mgr.ZSetUpsert(connID, db, key, member, score, memberBinary)
}

// ZSetRemoveMembers 有序集合删除。
func (a *App) ZSetRemoveMembers(connID string, db int, key string, members []string) error {
	return a.mgr.ZSetRemoveMembers(connID, db, key, members)
}

// StreamAdd 流消息追加。
func (a *App) StreamAdd(connID string, db int, key, id string, fields map[string]string) error {
	return a.mgr.StreamAdd(connID, db, key, id, fields)
}

// StreamDelete 流消息删除。
func (a *App) StreamDelete(connID string, db int, key string, ids []string) error {
	return a.mgr.StreamDelete(connID, db, key, ids)
}

// JSONSet JSON 文档写入。
func (a *App) JSONSet(connID string, db int, key, doc string) error {
	return a.mgr.JSONSet(connID, db, key, doc)
}

// DeleteKeys 删除键（backup=true 进回收站）。
func (a *App) DeleteKeys(connID string, db int, keys []string, backup bool) (*rds.DeleteResult, error) {
	return a.mgr.DeleteKeys(connID, db, keys, backup)
}

// RenameKey 重命名。
func (a *App) RenameKey(connID string, db int, oldKey, newKey string) error {
	return a.mgr.RenameKey(connID, db, oldKey, newKey)
}

// SetTTL 批量 TTL（ttlMs=-2 表示移除）。
func (a *App) SetTTL(connID string, db int, keys []string, ttlMs int64) (*rds.TTLResult, error) {
	return a.mgr.SetTTL(connID, db, keys, ttlMs)
}

// RecycleList 回收站列表。
func (a *App) RecycleList(connID string) ([]rds.RecycleItem, error) {
	return a.mgr.RecycleList(connID)
}

// RecycleRestore 回收站恢复。
func (a *App) RecycleRestore(connID, token string) error {
	return a.mgr.RecycleRestore(connID, token)
}

// RecycleRemove 彻底移除回收站条目。
func (a *App) RecycleRemove(connID, token string) error {
	return a.mgr.RecycleRemove(connID, token)
}

// RecycleClear 清空回收站。
func (a *App) RecycleClear(connID string) error {
	return a.mgr.RecycleClear(connID)
}

// ---------- 监控 ----------

// MonitorHistory 历史采样（seconds 窗口）。
func (a *App) MonitorHistory(connID string, seconds int) ([]rds.Sample, error) {
	b, err := a.mgr.Get(connID)
	if err != nil {
		return nil, err
	}
	if b.Monitor == nil {
		return []rds.Sample{}, nil
	}
	return b.Monitor.History(seconds), nil
}

// ---------- 控制台 ----------

// ExecCommand 执行命令（confirm 表示已二次确认）。
func (a *App) ExecCommand(connID, line string, confirm bool) (*rds.ConsoleResult, error) {
	return a.mgr.ExecCommand(connID, line, confirm)
}

// CompleteKeys 键名补全。
func (a *App) CompleteKeys(connID string, db int, prefix string) ([]string, error) {
	return a.mgr.CompleteCommand(connID, db, prefix)
}

// CommandHelpList 命令帮助表（补全用）。
func (a *App) CommandHelpList() []rds.CommandHelp {
	return rds.ExportCommandHelp()
}

// ---------- 分析 ----------

// BeginBigKeyScan 启动大 Key 扫描。
func (a *App) BeginBigKeyScan(connID string, topN int, maxScan int64) (string, error) {
	return a.mgr.BeginBigKeyScan(connID, topN, maxScan)
}

// StopBigKeyScan 停止扫描。
func (a *App) StopBigKeyScan(taskID string) {
	a.mgr.StopBigKeyScan(taskID)
}

// GetSlowLog 慢查询。
func (a *App) GetSlowLog(connID string, limit int64) ([]rds.SlowLogEntry, error) {
	return a.mgr.GetSlowLog(connID, limit)
}

// GetCommandStats 热命令统计。
func (a *App) GetCommandStats(connID string) ([]rds.CmdStat, error) {
	return a.mgr.GetCommandStats(connID)
}

// ---------- 导入导出 ----------

// PreviewDelete 批量删除预览。
func (a *App) PreviewDelete(connID string, db int, match, typeFilter string, maxPreview int) (*rds.DeletePreview, error) {
	return a.mgr.PreviewDelete(connID, db, match, typeFilter, maxPreview)
}

// ExportKeys JSON 导出。
func (a *App) ExportKeys(connID string, db int, match, typeFilter, filePath string) (*rds.ExportReport, error) {
	return a.mgr.ExportKeys(connID, db, match, typeFilter, filePath)
}

// ExportCSV CSV 导出。
func (a *App) ExportCSV(connID string, db int, match, typeFilter, filePath string) (*rds.ExportReport, error) {
	return a.mgr.ExportCSV(connID, db, match, typeFilter, filePath)
}

// ImportFile 导入。
func (a *App) ImportFile(connID string, db int, filePath string, overwrite bool) (*rds.ImportReport, error) {
	return a.mgr.ImportFile(connID, db, filePath, overwrite)
}

// ---------- 发布订阅 ----------

// PubSubSubscribe 订阅。
func (a *App) PubSubSubscribe(connID string, channels, patterns []string) error {
	return a.mgr.PubSubSubscribe(connID, channels, patterns)
}

// PubSubUnsubscribe 取消订阅。
func (a *App) PubSubUnsubscribe(connID string) error {
	return a.mgr.PubSubUnsubscribe(connID)
}

// Publish 发布消息。
func (a *App) Publish(connID, channel, payload string) (int64, error) {
	return a.mgr.Publish(connID, channel, payload)
}

// ---------- 对话框 ----------

// PickSaveFile 保存文件对话框。
func (a *App) PickSaveFile(title, defaultName string) (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: defaultName,
	})
}

// PickOpenFile 打开文件对话框。
func (a *App) PickOpenFile(title string) (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: title})
}

// ---------- 内部 ----------

func (a *App) openLogFile(cfgDir string) {
	logDir := filepath.Join(cfgDir, "logs")
	_ = os.MkdirAll(logDir, 0o700)
	f, err := os.OpenFile(filepath.Join(logDir, "redislens.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //#nosec G304 -- 固定文件名，目录为应用配置目录
	if err != nil {
		return
	}
	a.logFile = f
	log.SetOutput(f)
}

func (a *App) closeLogFile() {
	if a.logFile != nil {
		_ = a.logFile.Close()
		a.logFile = nil
	}
	log.SetOutput(os.Stderr)
}

// appConfigDir 返回配置目录：%AppData%/RedisLens。
func appConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, config.AppName), nil
}
