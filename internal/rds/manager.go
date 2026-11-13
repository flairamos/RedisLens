package rds

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"RedisLens/internal/audit"
	"RedisLens/internal/config"
)

// Health 健康状态。
type Health string

const (
	HealthConnecting Health = "connecting"
	HealthOK         Health = "ok"
	HealthDown       Health = "down"
)

// Bundle 一个已打开连接的运行时聚合：客户端、SSH 拨号器、监控器、回收站等。
type Bundle struct {
	ID       string
	Cfg      config.Connection
	Client   redis.UniversalClient
	Dialer   *sshDialer
	Monitor  *Monitor
	Recycle  *RecycleBin
	ReadOnly bool
	pubsub   *PubSubManager

	mu        sync.RWMutex
	health    Health
	lastErr   string
	summary   *ServerSummary
	dialFail  int // 连续拨号失败次数
	cancel    context.CancelFunc
	dbCache   map[int]redis.UniversalClient // 多 DB 独立客户端缓存
	dbCacheMu sync.Mutex
}

func (b *Bundle) Health() (Health, string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.health, b.lastErr
}

func (b *Bundle) setHealth(h Health, err string) {
	b.mu.Lock()
	b.health = h
	b.lastErr = err
	b.mu.Unlock()
}

func (b *Bundle) Summary() *ServerSummary {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.summary
}

func (b *Bundle) setSummary(s *ServerSummary) {
	b.mu.Lock()
	b.summary = s
	b.mu.Unlock()
}

// Manager 连接管理器：持有全部活动连接，负责心跳探活与自动重连。
type Manager struct {
	mu      sync.RWMutex
	bundles map[string]*Bundle
	tasks   map[string]*taskEntry // 后台分析任务
	store   *config.Store
	audit   *audit.Logger
	emit    func(event string, payload any)
	cancel  context.CancelFunc
}

// NewManager 创建管理器。emit 用于向前端推送事件。
func NewManager(store *config.Store, al *audit.Logger, emit func(event string, payload any)) *Manager {
	if emit == nil {
		emit = func(string, any) {}
	}
	return &Manager{
		bundles: make(map[string]*Bundle),
		store:   store,
		audit:   al,
		emit:    emit,
	}
}

// EmitEvent 暴露事件通道给其他子系统（监控采样推送等）。
func (m *Manager) EmitEvent(event string, payload any) { m.emit(event, payload) }

// Open 打开（或复用）一个连接。
func (m *Manager) Open(ctx context.Context, cfg *config.Connection) (*ServerSummary, error) {
	m.mu.Lock()
	if b, ok := m.bundles[cfg.ID]; ok {
		m.mu.Unlock()
		if s := b.Summary(); s != nil {
			return s, nil
		}
		return nil, errors.New("connection is warming up")
	}
	rootCtx, cancel := context.WithCancel(context.Background())
	b := &Bundle{
		ID:       cfg.ID,
		Cfg:      *cfg,
		Recycle:  NewRecycleBin(),
		ReadOnly: cfg.ReadOnly,
		health:   HealthConnecting,
		cancel:   cancel,
	}
	m.bundles[cfg.ID] = b
	m.mu.Unlock()

	summary, openErr := m.openBundle(ctx, b)
	m.mu.Lock()
	m.bundles[cfg.ID] = b
	m.mu.Unlock()

	// 心跳探活
	go m.healthLoop(rootCtx, b)

	if openErr == nil {
		// 启动监控采样
		interval := m.store.Settings().MonitorIntervalSec
		if interval < 1 {
			interval = 10
		}
		b.Monitor = NewMonitor(b.ID, b.Client, time.Duration(interval)*time.Second, m.emit)
		b.Monitor.Start()
		m.audit.Log(audit.Entry{
			ConnID: cfg.ID, ConnName: cfg.Name, Category: "connection",
			Action: "open", Target: encodeTarget(cfg.Host, cfg.Port), Result: "ok", Level: "info",
		})
		m.store.TouchConnection(cfg.ID)
	} else {
		m.audit.Log(audit.Entry{
			ConnID: cfg.ID, ConnName: cfg.Name, Category: "connection",
			Action: "open", Target: encodeTarget(cfg.Host, cfg.Port), Result: "error",
			Detail: openErr.Error(), Level: "warn",
		})
	}
	return summary, openErr
}

// openBundle 实际建立客户端连接（含自动重连退避）。
func (m *Manager) openBundle(ctx context.Context, b *Bundle) (*ServerSummary, error) {
	cfg := &b.Cfg
	var dialer dialFunc
	if cfg.SSHTunnel.Enabled {
		b.Dialer = newSSHDialer(&cfg.SSHTunnel)
		dialer = b.Dialer.Dial
	}
	client, err := buildClient(cfg, dialer)
	if err != nil {
		b.setHealth(HealthDown, err.Error())
		return nil, err
	}
	b.Client = client
	if err := client.Ping(ctx).Err(); err != nil {
		b.setHealth(HealthDown, err.Error())
		return nil, fmt.Errorf("ping %s: %w", encodeTarget(cfg.Host, cfg.Port), err)
	}
	summary, err := summarize(ctx, client, cfg.Mode)
	if err != nil {
		summary = &ServerSummary{Version: "?", Mode: cfg.Mode, TotalKeys: -1}
	}
	b.setSummary(summary)
	b.setHealth(HealthOK, "")
	return summary, nil
}

// healthLoop 心跳探活：每 5 秒 PING，断线自动重建客户端。
func (m *Manager) healthLoop(ctx context.Context, b *Bundle) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.probeOnce(ctx, b)
		}
	}
}

func (m *Manager) probeOnce(ctx context.Context, b *Bundle) {
	if b.Client == nil {
		m.reopen(ctx, b, "client is nil")
		return
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err := b.Client.Ping(pingCtx).Err()
	cancel()
	if err == nil {
		if h, _ := b.Health(); h != HealthOK {
			b.setHealth(HealthOK, "")
			if s, err := summarize(ctx, b.Client, b.Cfg.Mode); err == nil {
				b.setSummary(s)
			}
			m.emit("conn:health", map[string]any{"connId": b.ID, "status": HealthOK, "error": ""})
			log.Printf("[conn %s] reconnected", b.ID)
		}
		return
	}
	// 断线
	health, _ := b.Health()
	b.setHealth(HealthDown, err.Error())
	if health != HealthDown {
		m.emit("conn:health", map[string]any{"connId": b.ID, "status": HealthDown, "error": err.Error()})
		m.audit.Log(audit.Entry{
			ConnID: b.ID, ConnName: b.Cfg.Name, Category: "connection",
			Action: "disconnect", Result: "error", Detail: err.Error(), Level: "warn",
		})
	}
	m.reopen(ctx, b, err.Error())
}

// reopen 重建客户端（SSH 拨号器与 redis 客户端整体重建）。
// 上下文已取消（连接已关闭）时直接退出，避免关闭后重建泄漏。
func (m *Manager) reopen(ctx context.Context, b *Bundle, reason string) {
	if ctx.Err() != nil {
		return
	}
	m.mu.Lock()
	b.dialFail++
	attempt := b.dialFail
	m.mu.Unlock()
	if attempt > 30 {
		return // 放弃自动重连，等待用户手动操作
	}
	time.Sleep(backoff(attempt))
	if ctx.Err() != nil {
		return
	}
	// 优雅关闭旧客户端
	if old := b.Client; old != nil {
		_ = old.Close()
		b.Client = nil
	}
	b.closeDBCaches()
	if b.Dialer != nil {
		b.Dialer.Close()
		b.Dialer = nil
	}
	_, err := m.openBundle(ctx, b)
	if err == nil {
		m.mu.Lock()
		b.dialFail = 0
		m.mu.Unlock()
		if b.Monitor != nil {
			b.Monitor.SetClient(b.Client)
		}
		m.emit("conn:health", map[string]any{"connId": b.ID, "status": HealthOK, "error": ""})
	}
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * time.Second
	if d > 15*time.Second {
		d = 15 * time.Second
	}
	return d
}

// Get 获取活动连接。
func (m *Manager) Get(id string) (*Bundle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.bundles[id]
	if !ok {
		return nil, fmt.Errorf("connection %s is not open", id)
	}
	return b, nil
}

// IsOpen 判断连接是否已打开。
func (m *Manager) IsOpen(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.bundles[id]
	return ok
}

// ListOpen 返回所有活动连接 ID。
func (m *Manager) ListOpen() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.bundles))
	for id := range m.bundles {
		out = append(out, id)
	}
	return out
}

// Close 关闭一个连接并释放全部资源。
func (m *Manager) Close(id string) {
	// 取消该连接的后台分析任务
	m.mu.Lock()
	for tid, te := range m.tasks {
		te.cancel()
		delete(m.tasks, tid)
	}
	b, ok := m.bundles[id]
	if ok {
		delete(m.bundles, id)
	}
	m.mu.Unlock()
	if !ok {
		return
	}
	if b.Monitor != nil {
		b.Monitor.Stop()
	}
	if b.pubsub != nil {
		b.pubsub.Unsubscribe()
	}
	b.closeDBCaches()
	if b.Client != nil {
		_ = b.Client.Close()
	}
	if b.Dialer != nil {
		b.Dialer.Close()
	}
	b.cancel()
}

// CloseAll 关闭全部连接。
func (m *Manager) CloseAll() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.bundles))
	for id := range m.bundles {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		m.Close(id)
	}
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()
}

// Client 获取某连接的 redis 客户端（快捷方法）。
func (m *Manager) Client(id string) (redis.UniversalClient, *Bundle, error) {
	b, err := m.Get(id)
	if err != nil {
		return nil, nil, err
	}
	if b.Client == nil {
		return nil, b, errors.New("connection is down")
	}
	return b.Client, b, nil
}
