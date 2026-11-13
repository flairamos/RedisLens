package rds

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Sample 一帧监控采样。
type Sample struct {
	Time        int64   `json:"time"` // unix ms
	QPS         float64 `json:"qps"`
	HitRate     float64 `json:"hitRate"`
	HitsRate    float64 `json:"hitsRate"`    // 命中 次/秒
	MissRate    float64 `json:"missRate"`    // 未命中 次/秒
	UsedMem     uint64  `json:"usedMem"`
	MaxMem      uint64  `json:"maxMem"`
	RSSMem      uint64  `json:"rssMem"`
	FragRatio   float64 `json:"fragRatio"`
	Clients     int     `json:"clients"`
	Blocked     int     `json:"blocked"`
	NetInRate   float64 `json:"netInRate"` // bytes/s
	NetOutRate  float64 `json:"netOutRate"`
	TotalKeys   int64   `json:"totalKeys"`
	ExpiredRate float64 `json:"expiredRate"`
	EvictedRate float64 `json:"evictedRate"`
	CPUUserRate float64 `json:"cpuUserRate"`
	CPUSysRate  float64 `json:"cpuSysRate"`
	Role        string  `json:"role"`
	Slaves      int     `json:"slaves"`
	ReplLag     int64   `json:"replLag"` // 主从延迟（字节差）
	MasterLink  string  `json:"masterLink"`
	Version     string  `json:"version"`
}

// prevSnapshot 用于差分计算的上一次累计值。
type prevSnapshot struct {
	time       time.Time
	commands   float64
	netIn      float64
	netOut     float64
	hits       float64
	misses     float64
	expired    float64
	evicted    float64
	cpuUser    float64
	cpuSys     float64
	valid      bool
}

// Monitor INFO 周期采样器，环形缓冲保留最近 N 帧支持历史回看。
type Monitor struct {
	connID   string
	interval time.Duration
	maxKeep  int

	mu      sync.Mutex
	samples []Sample
	prev    prevSnapshot
	client  redis.UniversalClient

	ctx    context.Context
	cancel context.CancelFunc
	emit   func(event string, payload any)
	once   sync.Once
}

// NewMonitor 创建监控器。
func NewMonitor(connID string, client redis.UniversalClient, interval time.Duration, emit func(string, any)) *Monitor {
	if emit == nil {
		emit = func(string, any) {}
	}
	return &Monitor{
		connID:   connID,
		interval: interval,
		maxKeep:  8640, // 24 小时 @10s
		client:   client,
		emit:     emit,
	}
}

// Start 启动采样循环。
func (m *Monitor) Start() {
	if m.client == nil {
		return
	}
	m.once.Do(func() {
		m.ctx, m.cancel = context.WithCancel(context.Background())
		go m.loop()
	})
}

// Stop 停止采样。
func (m *Monitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// SetInterval 动态调整采样间隔。
func (m *Monitor) SetInterval(d time.Duration) {
	if d < time.Second {
		d = time.Second
	}
	m.mu.Lock()
	m.interval = d
	m.mu.Unlock()
}

// SetClient 重连后替换客户端。
func (m *Monitor) SetClient(c redis.UniversalClient) {
	m.mu.Lock()
	m.client = c
	m.prev.valid = false
	m.mu.Unlock()
}

// Interval 当前采样间隔。
func (m *Monitor) Interval() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.interval
}

func (m *Monitor) loop() {
	// 立即首采
	m.sampleOnce()
	for {
		m.mu.Lock()
		interval := m.interval
		m.mu.Unlock()
		// 用定时器 + ctx 双路 select：Stop() 时立即退出，不等待当前周期结束
		timer := time.NewTimer(interval)
		select {
		case <-m.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		m.sampleOnce()
	}
}

func (m *Monitor) sampleOnce() {
	m.mu.Lock()
	client := m.client
	m.mu.Unlock()
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(m.ctx, 8*time.Second)
	defer cancel()
	info, err := client.Info(ctx).Result()
	if err != nil {
		// 断线期间静默，等 healthLoop 重连
		return
	}
	kv := parseInfo(info)
	now := time.Now()

	s := Sample{
		Time:      now.UnixMilli(),
		UsedMem:   parseUint(kv["used_memory"]),
		MaxMem:    parseUint(kv["maxmemory"]),
		RSSMem:    parseUint(kv["used_memory_rss"]),
		FragRatio: parseFloat(kv["mem_fragmentation_ratio"]),
		Clients:   int(parseInt(kv["connected_clients"])),
		Blocked:   int(parseInt(kv["blocked_clients"])),
		Role:      kv["role"],
		Version:   kv["redis_version"],
		MasterLink: kv["master_link_status"],
		Slaves:    int(parseInt(kv["connected_slaves"])),
	}
	if dbsize, err := client.DBSize(ctx).Result(); err == nil {
		s.TotalKeys = dbsize
	}

	m.mu.Lock()
	prev := m.prev
	dt := now.Sub(prev.time).Seconds()
	if prev.valid && dt > 0 {
		s.QPS = (parseFloat(kv["total_commands_processed"]) - prev.commands) / dt
		s.NetInRate = (parseFloat(kv["total_net_input_bytes"]) - prev.netIn) / dt
		s.NetOutRate = (parseFloat(kv["total_net_output_bytes"]) - prev.netOut) / dt
		hits := parseFloat(kv["keyspace_hits"]) - prev.hits
		misses := parseFloat(kv["keyspace_misses"]) - prev.misses
		s.HitsRate = hits / dt
		s.MissRate = misses / dt
		if hits+misses > 0 {
			s.HitRate = hits / (hits + misses) * 100
		}
		s.ExpiredRate = (parseFloat(kv["expired_keys"]) - prev.expired) / dt
		s.EvictedRate = (parseFloat(kv["evicted_keys"]) - prev.evicted) / dt
		s.CPUUserRate = (parseFloat(kv["used_cpu_user"]) - prev.cpuUser) / dt
		s.CPUSysRate = (parseFloat(kv["used_cpu_sys"]) - prev.cpuSys) / dt
	} else {
		// 首帧：以累计值展示瞬时命中率
		hits := parseFloat(kv["keyspace_hits"])
		misses := parseFloat(kv["keyspace_misses"])
		if hits+misses > 0 {
			s.HitRate = hits / (hits + misses) * 100
		}
	}
	// 主从延迟：从节点视角 master_repl_offset 与本机 offset 差值
	if s.Role == "slave" || s.Role == "replica" {
		s.ReplLag = parseInt(kv["master_repl_offset"]) - parseInt(kv["slave_repl_offset"])
	}

	m.prev = prevSnapshot{
		time: now, valid: true,
		commands: parseFloat(kv["total_commands_processed"]),
		netIn:    parseFloat(kv["total_net_input_bytes"]),
		netOut:   parseFloat(kv["total_net_output_bytes"]),
		hits:     parseFloat(kv["keyspace_hits"]),
		misses:   parseFloat(kv["keyspace_misses"]),
		expired:  parseFloat(kv["expired_keys"]),
		evicted:  parseFloat(kv["evicted_keys"]),
		cpuUser:  parseFloat(kv["used_cpu_user"]),
		cpuSys:   parseFloat(kv["used_cpu_sys"]),
	}
	m.samples = append(m.samples, s)
	if len(m.samples) > m.maxKeep {
		m.samples = m.samples[len(m.samples)-m.maxKeep:]
	}
	m.mu.Unlock()

	m.emit("monitor:sample", map[string]any{"connId": m.connID, "sample": s})
}

// History 返回最近 seconds 秒内的采样帧。
func (m *Monitor) History(seconds int) []Sample {
	m.mu.Lock()
	defer m.mu.Unlock()
	if seconds <= 0 {
		seconds = 3600
	}
	cutoff := time.Now().Add(-time.Duration(seconds) * time.Second).UnixMilli()
	out := make([]Sample, 0, len(m.samples))
	for _, s := range m.samples {
		if s.Time >= cutoff {
			out = append(out, s)
		}
	}
	return out
}

// Latest 返回最新一帧（可能为空）。
func (m *Monitor) Latest() (Sample, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.samples) == 0 {
		return Sample{}, false
	}
	return m.samples[len(m.samples)-1], true
}
