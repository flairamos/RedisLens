package rds

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"RedisLens/internal/audit"
)

// BigKeyItem 大 Key 排行条目。
type BigKeyItem struct {
	Key      string `json:"key"`
	Type     string `json:"type"`
	Elements int64  `json:"elements"` // 元素数量（list/hash/set/zset/stream 长度）
	Bytes    int64  `json:"bytes"`    // 内存占用
}

// BigKeyReport 大 Key 分析报告。
type BigKeyReport struct {
	ScannedKeys  int64        `json:"scannedKeys"`
	TotalKeys    int64        `json:"totalKeys"`
	DurationMs   int64        `json:"durationMs"`
	TopByMemory  []BigKeyItem `json:"topByMemory"`
	TopByElement []BigKeyItem `json:"topByElement"`
	TypeDist     map[string]int64 `json:"typeDist"`   // 类型分布
	TTLBuckets   map[string]int64 `json:"ttlBuckets"` // TTL 分布
	Stopped      bool         `json:"stopped"`
}

// taskRegistry 分析任务登记（可取消）。
type taskEntry struct {
	cancel context.CancelFunc
}

// BeginBigKeyScan 启动全库大 Key 扫描（后台执行，进度经事件推送）。
// 返回任务 ID。事件 "analysis:progress" {taskId, scanned, found, totalKeys}，
// 完成 "analysis:done" {taskId, report}。
func (m *Manager) BeginBigKeyScan(connID string, topN int, maxScan int64) (string, error) {
	b, err := m.Get(connID)
	if err != nil {
		return "", err
	}
	cli, _, err := m.dbClient(connID, b.Cfg.DB)
	if err != nil {
		return "", err
	}
	if topN <= 0 {
		topN = 20
	}
	if maxScan <= 0 {
		maxScan = 1000000
	}
	ctx, cancel := context.WithCancel(context.Background())
	taskID := fmt.Sprintf("bigkey-%d", time.Now().UnixNano())
	m.mu.Lock()
	if m.tasks == nil {
		m.tasks = make(map[string]*taskEntry)
	}
	m.tasks[taskID] = &taskEntry{cancel: cancel}
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.tasks, taskID)
			m.mu.Unlock()
			cancel()
		}()
		start := time.Now()
		report := &BigKeyReport{
			TopByMemory:  []BigKeyItem{},
			TopByElement: []BigKeyItem{},
			TypeDist:     map[string]int64{},
			TTLBuckets:   map[string]int64{},
		}
		if total, err := cli.DBSize(ctx).Result(); err == nil {
			report.TotalKeys = total
		}
		memTop := &topList{n: topN, less: func(a, b BigKeyItem) bool { return a.Bytes < b.Bytes }}
		elemTop := &topList{n: topN, less: func(a, b BigKeyItem) bool { return a.Elements < b.Elements }}

		var cursor uint64
		scanned := int64(0)
		lastEmit := time.Now()
		for {
			if ctx.Err() != nil {
				report.Stopped = true
				break
			}
			keys, next, err := cli.Scan(ctx, cursor, "", 200).Result()
			if err != nil {
				break
			}
			for _, k := range keys {
				it, ok := measureKey(ctx, cli, k)
				if !ok {
					continue
				}
				scanned++
				report.TypeDist[it.Type]++
				bucket := ttlBucket(ctx, cli, k)
				report.TTLBuckets[bucket]++
				memTop.push(it)
				elemTop.push(it)
			}
			if time.Since(lastEmit) >= 400*time.Millisecond {
				lastEmit = time.Now()
				m.emit("analysis:progress", map[string]any{
					"taskId": taskID, "connId": connID,
					"scanned": scanned, "totalKeys": report.TotalKeys,
				})
			}
			if next == 0 || scanned >= maxScan {
				break
			}
			cursor = next
		}
		report.ScannedKeys = scanned
		report.DurationMs = time.Since(start).Milliseconds()
		report.TopByMemory = memTop.items()
		report.TopByElement = elemTop.items()
		m.emit("analysis:done", map[string]any{"taskId": taskID, "connId": connID, "report": report})
		m.audit.Log(audit.Entry{
			ConnID: b.ID, ConnName: b.Cfg.Name, Category: "analysis",
			Action: "BIGKEY_SCAN", Result: "ok",
			Detail: fmt.Sprintf("scanned=%d duration=%dms", scanned, report.DurationMs), Level: "info",
		})
	}()
	return taskID, nil
}

// StopBigKeyScan 取消扫描任务。
func (m *Manager) StopBigKeyScan(taskID string) {
	m.mu.Lock()
	if t, ok := m.tasks[taskID]; ok {
		t.cancel()
	}
	m.mu.Unlock()
}

// measureKey 测量单个键的规模。
func measureKey(ctx context.Context, cli redis.UniversalClient, key string) (BigKeyItem, bool) {
	typ, err := cli.Type(ctx, key).Result()
	if err != nil || typ == "none" {
		return BigKeyItem{}, false
	}
	item := BigKeyItem{Key: key, Type: typ}
	if bytes, err := cli.MemoryUsage(ctx, key).Result(); err == nil {
		item.Bytes = bytes
	}
	switch typ {
	case "list":
		n, _ := cli.LLen(ctx, key).Result()
		item.Elements = n
	case "set":
		n, _ := cli.SCard(ctx, key).Result()
		item.Elements = n
	case "zset":
		n, _ := cli.ZCard(ctx, key).Result()
		item.Elements = n
	case "hash":
		n, _ := cli.HLen(ctx, key).Result()
		item.Elements = n
	case "stream":
		n, _ := cli.XLen(ctx, key).Result()
		item.Elements = n
	case "string":
		n, _ := cli.StrLen(ctx, key).Result()
		item.Elements = n
	}
	return item, true
}

// ttlBucket TTL 分布桶。
func ttlBucket(ctx context.Context, cli redis.UniversalClient, key string) string {
	ttl, err := cli.PTTL(ctx, key).Result()
	if err != nil {
		return "error"
	}
	switch {
	case ttl == -1:
		return "永不过期"
	case ttl <= 60_000:
		return "≤1分钟"
	case ttl <= 3_600_000:
		return "≤1小时"
	case ttl <= 86_400_000:
		return "≤1天"
	case ttl <= 7*86_400_000:
		return "≤7天"
	case ttl <= 30*86_400_000:
		return "≤30天"
	default:
		return ">30天"
	}
}

// topList 定长 TopN 列表。
type topList struct {
	n    int
	data []BigKeyItem
	less func(a, b BigKeyItem) bool
}

func (t *topList) push(item BigKeyItem) {
	t.data = append(t.data, item)
	sort.Slice(t.data, func(i, j int) bool { return t.less(t.data[i], t.data[j]) })
	if len(t.data) > t.n {
		t.data = t.data[len(t.data)-t.n:]
	}
}

func (t *topList) items() []BigKeyItem {
	out := make([]BigKeyItem, len(t.data))
	copy(out, t.data)
	// 倒序：最大在前
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// SlowLogEntry 慢查询条目。
type SlowLogEntry struct {
	ID        int64    `json:"id"`
	Time      int64    `json:"time"`      // unix 秒
	Duration  int64    `json:"duration"`  // 微秒
	Command   string   `json:"command"`   // 完整命令
	Client    string   `json:"client"`
	ClientName string  `json:"clientName"`
}

// GetSlowLog 查询慢查询日志。
func (m *Manager) GetSlowLog(connID string, limit int64) ([]SlowLogEntry, error) {
	b, err := m.Get(connID)
	if err != nil {
		return nil, err
	}
	cli, _, err := m.dbClient(connID, b.Cfg.DB)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if limit <= 0 {
		limit = 100
	}
	logs, err := cli.SlowLogGet(ctx, limit).Result()
	if err != nil {
		return nil, err
	}
	out := make([]SlowLogEntry, 0, len(logs))
	for _, l := range logs {
		entry := SlowLogEntry{
			ID:         l.ID,
			Time:       l.Time.Unix(),
			Duration:   int64(l.Duration),
			Client:     l.ClientAddr,
			ClientName: l.ClientName,
		}
		parts := make([]string, 0, len(l.Args))
		for _, a := range l.Args {
			parts = append(parts, fmt.Sprintf("%v", a))
		}
		entry.Command = strings.Join(parts, " ")
		out = append(out, entry)
	}
	return out, nil
}

// GetCommandStats 从 INFO commandstats 提取热命令排行（热 Key 场景的近似观测）。
func (m *Manager) GetCommandStats(connID string) ([]CmdStat, error) {
	b, err := m.Get(connID)
	if err != nil {
		return nil, err
	}
	cli := b.Client
	if cli == nil {
		return nil, fmt.Errorf("connection is down")
	}
	ctx, cancel := opCtx()
	defer cancel()
	info, err := cli.Info(ctx, "commandstats").Result()
	if err != nil {
		// 低版本不支持 section 参数时降级为全量 INFO
		info, err = cli.Info(ctx).Result()
		if err != nil {
			return nil, err
		}
	}
	var out []CmdStat
	for k, v := range parseInfo(info) {
		// cmdstat_get:calls=100,usec=1234,usec_per_call=12.3
		if !strings.HasPrefix(k, "cmdstat_") {
			continue
		}
		name := strings.TrimPrefix(k, "cmdstat_")
		st := CmdStat{Command: name}
		for _, part := range strings.Split(v, ",") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			switch kv[0] {
			case "calls":
				st.Calls = parseInt(kv[1])
			case "usec":
				st.Usec = parseInt(kv[1])
			}
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Calls > out[j].Calls })
	if len(out) > 30 {
		out = out[:30]
	}
	return out, nil
}

// CmdStat 命令统计。
type CmdStat struct {
	Command string `json:"command"`
	Calls   int64  `json:"calls"`
	Usec    int64  `json:"usec"`
}
