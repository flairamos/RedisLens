package rds

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/redis/go-redis/v9"
)

// KeyItem 键列表条目。
type KeyItem struct {
	Key  string `json:"key"`
	Type string `json:"type"`
	TTL  int64  `json:"ttl"` // 剩余毫秒；-1 永久；-2 不存在
	Size int64  `json:"size"`
}

// ScanPageResp SCAN 分页结果。
type ScanPageResp struct {
	Cursor uint64    `json:"cursor"`
	Keys   []KeyItem `json:"keys"`
	Done   bool      `json:"done"` // 游标回到 0，遍历结束
	Scanned int      `json:"scanned"`
}

const (
	keySep          = ":"    // 键名分隔符
	treeMaxScanKeys = 300000 // 树构建最多扫描的键数（防失控）
	treeMaxNodes    = 2000   // 单层最大返回节点数
)

// dbClient 获取指定 DB 的客户端。cluster 固定 db0；standalone 按 DB 缓存独立客户端。
func (m *Manager) dbClient(id string, db int) (redis.UniversalClient, *Bundle, error) {
	b, err := m.Get(id)
	if err != nil {
		return nil, nil, err
	}
	if b.Client == nil {
		return nil, b, fmt.Errorf("connection %s is down", id)
	}
	if b.Cfg.Mode == "cluster" || db == b.Cfg.DB {
		return b.Client, b, nil
	}
	b.dbCacheMu.Lock()
	defer b.dbCacheMu.Unlock()
	if c, ok := b.dbCache[db]; ok {
		return c, b, nil
	}
	cfg := b.Cfg
	cfg.DB = db
	cli, err := buildClient(&cfg, b.Dialer.Dial)
	if err != nil {
		return nil, b, err
	}
	if b.dbCache == nil {
		b.dbCache = make(map[int]redis.UniversalClient)
	}
	b.dbCache[db] = cli
	return cli, b, nil
}

// closeDBCaches 关闭并清空多 DB 缓存客户端（重连时旧拨号器已失效）。
func (b *Bundle) closeDBCaches() {
	b.dbCacheMu.Lock()
	for _, c := range b.dbCache {
		_ = c.Close()
	}
	b.dbCache = nil
	b.dbCacheMu.Unlock()
}

// ScanPage SCAN 游标分页遍历键，附带类型与 TTL（pipeline 批量查询）。
func (m *Manager) ScanPage(connID string, db int, cursor uint64, match, typeFilter string, count int) (*ScanPageResp, error) {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return nil, err
	}
	if count <= 0 {
		count = 500
	}
	ctx, cancel := opCtx()
	defer cancel()

	// SCAN 保证不阻塞：COUNT 限制每轮扫描量
	var keys []string
	var next uint64
	var scanned int
	keys, next, err = scanWithType(ctx, cli, cursor, match, typeFilter, int64(count))
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	scanned = len(keys)
	resp := &ScanPageResp{Cursor: next, Keys: make([]KeyItem, 0, len(keys)), Done: next == 0}
	for _, k := range keys {
		resp.Keys = append(resp.Keys, KeyItem{Key: k})
	}
	m.fillKeyMeta(ctx, cli, resp.Keys, b)
	_ = scanned
	return resp, nil
}

// fillKeyMeta pipeline 填充类型 / TTL / 元素规模。
func (m *Manager) fillKeyMeta(ctx context.Context, cli redis.UniversalClient, items []KeyItem, b *Bundle) {
	if len(items) == 0 {
		return
	}
	pipe := cli.Pipeline()
	typeCmds := make([]*redis.StatusCmd, len(items))
	ttlCmds := make([]*redis.DurationCmd, len(items))
	memCmds := make([]*redis.IntCmd, len(items))
	for i, it := range items {
		typeCmds[i] = pipe.Type(ctx, it.Key)
		ttlCmds[i] = pipe.PTTL(ctx, it.Key)
		memCmds[i] = pipe.MemoryUsage(ctx, it.Key)
	}
	_, _ = pipe.Exec(ctx)
	for i := range items {
		if t, err := typeCmds[i].Result(); err == nil {
			items[i].Type = normalizeType(t)
		}
		if d, err := ttlCmds[i].Result(); err == nil {
			items[i].TTL = ttlToMs(d)
		} else {
			items[i].TTL = -1
		}
		if n, err := memCmds[i].Result(); err == nil {
			items[i].Size = n
		} else {
			items[i].Size = -1
		}
	}
}

// normalizeType 统一类型名。
func normalizeType(t string) string {
	switch t {
	case "ReJSON-RL":
		return "json"
	case "MBbloop--":
		return "module"
	default:
		return t
	}
}

// TreeNode 键空间树节点：folder 或 key。
type TreeNode struct {
	Name     string `json:"name"`     // 展示名（最后一段）
	Prefix   string `json:"prefix"`   // folder: 完整前缀 key: 完整键名
	IsFolder bool   `json:"isFolder"`
	Type     string `json:"type,omitempty"`
	TTL      int64  `json:"ttl,omitempty"`
}

// KeyTree 构建指定前缀下的下一层树节点（folder + 叶子键）。
// 使用 SCAN MATCH prefix* 渐进扫描并聚合，扫描量有上限保护。
func (m *Manager) KeyTree(connID string, db int, prefix string) ([]TreeNode, error) {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()

	// 空前缀（根）与具体前缀分别处理匹配模式
	sepPrefix := ""
	match := "*"
	if prefix != "" {
		sepPrefix = prefix + keySep
		match = sepPrefix + "*"
	}
	folders := map[string]bool{}
	leafs := map[string]string{} // key -> type
	var cursor uint64
	totalScanned := 0
	for {
		var keys []string
		var next uint64
		var e error
		keys, next, e = cli.Scan(ctx, cursor, match, 1000).Result()
		if e != nil {
			return nil, e
		}
		for _, k := range keys {
			rel := strings.TrimPrefix(k, sepPrefix)
			if idx := strings.Index(rel, keySep); idx > 0 {
				folders[rel[:idx]] = true
			} else {
				leafs[k] = ""
				if len(leafs) > treeMaxNodes {
					break
				}
			}
		}
		totalScanned += len(keys)
		if next == 0 || totalScanned >= treeMaxScanKeys ||
			len(folders)+len(leafs) >= treeMaxNodes {
			break
		}
		cursor = next
	}

	nodes := make([]TreeNode, 0, len(folders)+len(leafs))
	for name := range folders {
		fp := name
		if prefix != "" {
			fp = prefix + keySep + name
		}
		nodes = append(nodes, TreeNode{
			Name:     name,
			Prefix:   fp,
			IsFolder: true,
		})
	}
	// 叶子键类型查询（限量）
	leafKeys := make([]string, 0, len(leafs))
	for k := range leafs {
		leafKeys = append(leafKeys, k)
	}
	sort.Strings(leafKeys)
	if len(leafKeys) > 0 {
		items := make([]KeyItem, len(leafKeys))
		for i, k := range leafKeys {
			items[i] = KeyItem{Key: k}
		}
		m.fillKeyMeta(ctx, cli, items, b)
		for _, it := range items {
			nodes = append(nodes, TreeNode{
				Name:     it.Key[strings.LastIndex(it.Key, keySep)+1:],
				Prefix:   it.Key,
				IsFolder: false,
				Type:     it.Type,
				TTL:      it.TTL,
			})
		}
	}
	// folder 优先，组内按名称排序
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsFolder != nodes[j].IsFolder {
			return nodes[i].IsFolder
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// CountKeys 统计匹配模式的键数量（SCAN 全遍历，带上限）。
func (m *Manager) CountKeys(connID string, db int, match, typeFilter string, maxScan int) (int64, error) {
	cli, _, err := m.dbClient(connID, db)
	if err != nil {
		return 0, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	var total int64
	var cursor uint64
	scanned := 0
	for {
		var keys []string
		var next uint64
		keys, next, err = scanWithType(ctx, cli, cursor, match, typeFilter, 1000)
		if err != nil {
			return total, err
		}
		total += int64(len(keys))
		scanned += len(keys)
		if next == 0 || (maxScan > 0 && scanned >= maxScan) {
			break
		}
		cursor = next
	}
	return total, nil
}
