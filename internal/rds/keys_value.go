package rds

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/redis/go-redis/v9"

	"RedisLens/internal/audit"
)

// opCtx 常规操作上下文（20s 超时）。
func opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 20*time.Second)
}

// bytesToVal 二进制安全地转换为可展示字符串。返回值第二个参数标记是否为 base64。
func bytesToVal(b []byte) (string, bool) {
	if len(b) == 0 {
		return "", false
	}
	if utf8.Valid(b) && !strings.ContainsRune(string(b), 0) {
		return string(b), false
	}
	return base64.StdEncoding.EncodeToString(b), true
}

var jsonTrimRe = regexp.MustCompile(`^\s*[[{]`)

// looksLikeJSON 判断文本是否是 JSON 文档（用于前端高亮与格式化）。
func looksLikeJSON(s string) bool {
	return jsonTrimRe.MatchString(s) && json.Valid([]byte(s))
}

// ValueData 值查看聚合结果。
type ValueData struct {
	Key      string `json:"key"`
	Type     string `json:"type"`
	Encoding string `json:"encoding,omitempty"`
	TTL      int64  `json:"ttl"`     // ms；-1 永久
	Size     int64  `json:"size"`    // 字节（MEMORY USAGE）
	Elements int64  `json:"elements"`
	Binary   bool   `json:"binary"`  // 字符串值是否为二进制（base64 展示）
	Str      *string          `json:"str,omitempty"`
	IsJSON   bool             `json:"isJson,omitempty"`
	Hash     []FieldItem      `json:"hash,omitempty"`
	List     []ListItem       `json:"list,omitempty"`
	Set      []string         `json:"set,omitempty"`
	ZSet     []ZSetItem       `json:"zset,omitempty"`
	Stream   []StreamEntry    `json:"stream,omitempty"`
}

// FieldItem 哈希字段。
type FieldItem struct {
	Field  string `json:"field"`
	Value  string `json:"value"`
	Binary bool   `json:"binary"`
}

// ListItem 列表元素。
type ListItem struct {
	Index  int64  `json:"index"`
	Value  string `json:"value"`
	Binary bool   `json:"binary"`
}

// ZSetItem 有序集合成员。
type ZSetItem struct {
	Rank   int64   `json:"rank"`
	Member string  `json:"member"`
	Score  float64 `json:"score"`
}

// StreamEntry 流消息。
type StreamEntry struct {
	ID     string            `json:"id"`
	Fields map[string]string `json:"fields"`
}

// GetValue 获取键的完整值（集合类取首页，防大键撑爆 UI）。
func (m *Manager) GetValue(connID string, db int, key string) (*ValueData, error) {
	cli, _, err := m.dbClient(connID, db)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()

	typ, err := cli.Type(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	typ = normalizeType(typ)
	if typ == "none" {
		return nil, fmt.Errorf("key %s does not exist", key)
	}
	vd := &ValueData{Key: key, Type: typ}
	if ttl, err := cli.PTTL(ctx, key).Result(); err == nil {
		vd.TTL = ttlToMs(ttl)
	}
	if size, err := cli.MemoryUsage(ctx, key).Result(); err == nil {
		vd.Size = size
	}
	if enc, err := cli.ObjectEncoding(ctx, key).Result(); err == nil {
		vd.Encoding = enc
	}

	switch typ {
	case "string":
		raw, err := cli.Get(ctx, key).Bytes()
		if err != nil {
			return nil, err
		}
		text, binary := bytesToVal(raw)
		vd.Binary = binary
		vd.Str = &text
		vd.IsJSON = !binary && looksLikeJSON(text)
		vd.Elements = int64(len(raw))
	case "hash":
		vd.Elements, _ = cli.HLen(ctx, key).Result()
		fields, err := cli.HGetAll(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		for f, v := range fields {
			text, binary := bytesToVal([]byte(v))
			vd.Hash = append(vd.Hash, FieldItem{Field: f, Value: text, Binary: binary})
		}
		sortFields(vd.Hash)
	case "list":
		vd.Elements, _ = cli.LLen(ctx, key).Result()
		vals, err := cli.LRange(ctx, key, 0, 499).Result()
		if err != nil {
			return nil, err
		}
		for i, v := range vals {
			text, binary := bytesToVal([]byte(v))
			vd.List = append(vd.List, ListItem{Index: int64(i), Value: text, Binary: binary})
		}
	case "set":
		vd.Elements, _ = cli.SCard(ctx, key).Result()
		vals, _, err := cli.SScan(ctx, key, 0, "", 500).Result()
		if err != nil {
			return nil, err
		}
		vd.Set = vals
		sort.Strings(vd.Set)
	case "zset":
		vd.Elements, _ = cli.ZCard(ctx, key).Result()
		vals, err := cli.ZRangeWithScores(ctx, key, 0, 499).Result()
		if err != nil {
			return nil, err
		}
		for i, z := range vals {
			vd.ZSet = append(vd.ZSet, ZSetItem{Rank: int64(i), Member: fmt.Sprintf("%v", z.Member), Score: z.Score})
		}
	case "stream":
		vd.Elements, _ = cli.XLen(ctx, key).Result()
		msgs, err := cli.XRangeN(ctx, key, "-", "+", 200).Result()
		if err != nil {
			return nil, err
		}
		for _, msg := range msgs {
			fields := make(map[string]string, len(msg.Values))
			for k, v := range msg.Values {
				text, _ := bytesToVal([]byte(fmt.Sprintf("%v", v)))
				fields[k] = text
			}
			vd.Stream = append(vd.Stream, StreamEntry{ID: msg.ID, Fields: fields})
		}
	case "json":
		raw, err := cli.Do(ctx, "JSON.GET", key, "$").Result()
		if err != nil {
			return nil, err
		}
		text, binary := bytesToVal([]byte(fmt.Sprintf("%v", raw)))
		vd.Binary = binary
		vd.Str = &text
		vd.IsJSON = !binary
	default:
		return nil, fmt.Errorf("unsupported key type: %s", typ)
	}
	return vd, nil
}

func sortFields(items []FieldItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].Field < items[j-1].Field; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// assertWritable 只读模式下的写操作防线（UI 隐藏入口，后端兜底拦截）。
func (m *Manager) assertWritable(b *Bundle, action string) error {
	if b.ReadOnly {
		return fmt.Errorf("连接处于只读模式，已拦截写操作：%s", action)
	}
	return nil
}

// SetString 写入字符串值。ttlMs: -1 保留原 TTL，-2 移除 TTL，>=0 设置 TTL。
func (m *Manager) SetString(connID string, db int, key, value string, binary bool, ttlMs int64) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "SET"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()

	var raw []byte
	if binary {
		raw, err = base64.StdEncoding.DecodeString(value)
		if err != nil {
			return fmt.Errorf("invalid base64 value: %w", err)
		}
	} else {
		raw = []byte(value)
	}
	// SET 会清除 TTL：ttlMs == -1 表示"保留原 TTL"，需先读取
	origTTL := int64(-2)
	if ttlMs == -1 {
		if d, err := cli.PTTL(ctx, key).Result(); err == nil && d > 0 {
			origTTL = d.Milliseconds()
		} else {
			origTTL = -2 // 原本就永久
		}
	}
	if err := cli.Set(ctx, key, raw, 0).Err(); err != nil {
		return err
	}
	switch {
	case ttlMs == -1 && origTTL >= 0: // 恢复原 TTL
		if err := cli.PExpire(ctx, key, time.Duration(origTTL)*time.Millisecond).Err(); err != nil {
			return err
		}
	case ttlMs == -2:
		if err := cli.Persist(ctx, key).Err(); err != nil {
			return err
		}
	case ttlMs >= 0:
		if err := cli.PExpire(ctx, key, time.Duration(ttlMs)*time.Millisecond).Err(); err != nil {
			return err
		}
	}
	m.auditKey(b, "key_edit", "SET", key, fmt.Sprintf("len=%d ttl=%d", len(raw), ttlMs))
	return nil
}

// HashSetField 写入哈希字段。
func (m *Manager) HashSetField(connID string, db int, key, field, value string, binary bool) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "HSET"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.HSet(ctx, key, field, rawValue(value, binary)).Err(); err != nil {
		return err
	}
	m.auditKey(b, "key_edit", "HSET", key, field)
	return nil
}

// HashDelFields 删除哈希字段。
func (m *Manager) HashDelFields(connID string, db int, key string, fields []string) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "HDEL"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.HDel(ctx, key, fields...).Err(); err != nil {
		return err
	}
	m.auditKey(b, "key_edit", "HDEL", key, strings.Join(fields, ","))
	return nil
}

// ListSetIndex 修改列表指定下标。
func (m *Manager) ListSetIndex(connID string, db int, key string, index int64, value string, binary bool) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "LSET"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.LSet(ctx, key, index, rawValue(value, binary)).Err(); err != nil {
		return err
	}
	m.auditKey(b, "key_edit", "LSET", key, strconv.FormatInt(index, 10))
	return nil
}

// ListPush 向列表推入元素。side: left|right。
func (m *Manager) ListPush(connID string, db int, key, side, value string, binary bool) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "LPUSH/RPUSH"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	v := rawValue(value, binary)
	var e error
	if side == "left" {
		e = cli.LPush(ctx, key, v).Err()
	} else {
		e = cli.RPush(ctx, key, v).Err()
	}
	if e != nil {
		return e
	}
	m.auditKey(b, "key_edit", strings.ToUpper(side)+"PUSH", key, "")
	return nil
}

// ListRemoveIndex 删除列表元素（LSET 哨兵方案：置位后 LREM，兼容所有版本）。
func (m *Manager) ListRemoveIndex(connID string, db int, key string, index int64) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "LREM"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	sentinel := fmt.Sprintf("\x00__redislens_del_%d__\x00", time.Now().UnixNano())
	if err := cli.LSet(ctx, key, index, sentinel).Err(); err != nil {
		return err
	}
	if err := cli.LRem(ctx, key, 1, sentinel).Err(); err != nil {
		return err
	}
	m.auditKey(b, "key_edit", "LREM", key, strconv.FormatInt(index, 10))
	return nil
}

// SetAddMembers 集合添加成员。
func (m *Manager) SetAddMembers(connID string, db int, key string, members []string) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "SADD"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.SAdd(ctx, key, members).Err(); err != nil {
		return err
	}
	m.auditKey(b, "key_edit", "SADD", key, strings.Join(members, ","))
	return nil
}

// SetRemoveMembers 集合删除成员。
func (m *Manager) SetRemoveMembers(connID string, db int, key string, members []string) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "SREM"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.SRem(ctx, key, members).Err(); err != nil {
		return err
	}
	m.auditKey(b, "key_edit", "SREM", key, strings.Join(members, ","))
	return nil
}

// ZSetUpsert 写入有序集合成员与分数。
func (m *Manager) ZSetUpsert(connID string, db int, key, member string, score float64, memberBinary bool) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "ZADD"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.ZAdd(ctx, key, redis.Z{Score: score, Member: rawValue(member, memberBinary)}).Err(); err != nil {
		return err
	}
	m.auditKey(b, "key_edit", "ZADD", key, fmt.Sprintf("%s=%v", member, score))
	return nil
}

// ZSetRemoveMembers 删除有序集合成员。
func (m *Manager) ZSetRemoveMembers(connID string, db int, key string, members []string) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "ZREM"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.ZRem(ctx, key, members).Err(); err != nil {
		return err
	}
	m.auditKey(b, "key_edit", "ZREM", key, strings.Join(members, ","))
	return nil
}

// StreamAdd 追加流消息。
func (m *Manager) StreamAdd(connID string, db int, key, id string, fields map[string]string) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "XADD"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if id == "" {
		id = "*"
	}
	if err := cli.XAdd(ctx, &redis.XAddArgs{Stream: key, ID: id, Values: fields}).Err(); err != nil {
		return err
	}
	m.auditKey(b, "key_edit", "XADD", key, id)
	return nil
}

// StreamDelete 删除流消息。
func (m *Manager) StreamDelete(connID string, db int, key string, ids []string) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "XDEL"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.XDel(ctx, key, ids...).Err(); err != nil {
		return err
	}
	m.auditKey(b, "key_edit", "XDEL", key, strings.Join(ids, ","))
	return nil
}

// JSONSet 写入 JSON 文档（RedisJSON 模块）。
func (m *Manager) JSONSet(connID string, db int, key, doc string) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "JSON.SET"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.Do(ctx, "JSON.SET", key, "$", doc).Err(); err != nil {
		return err
	}
	m.auditKey(b, "key_edit", "JSON.SET", key, "")
	return nil
}

// CreateKey 创建新键（按类型初始化）。initial 为初始值的 JSON 表示。
func (m *Manager) CreateKey(connID string, db int, key, typ string, initial string, ttlMs int64) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "create key"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()

	var e error
	switch typ {
	case "string":
		e = cli.Set(ctx, key, initial, 0).Err()
	case "hash":
		e = cli.HSet(ctx, key, "field1", initial).Err()
	case "list":
		e = cli.RPush(ctx, key, initial).Err()
	case "set":
		e = cli.SAdd(ctx, key, initial).Err()
	case "zset":
		e = cli.ZAdd(ctx, key, redis.Z{Score: 1, Member: initial}).Err()
	case "stream":
		e = cli.XAdd(ctx, &redis.XAddArgs{Stream: key, Values: map[string]any{"field1": initial}}).Err()
	case "json":
		if !json.Valid([]byte(initial)) {
			return fmt.Errorf("invalid json document")
		}
		e = cli.Do(ctx, "JSON.SET", key, "$", initial).Err()
	default:
		return fmt.Errorf("unsupported type: %s", typ)
	}
	if e != nil {
		return e
	}
	if ttlMs >= 0 {
		if e = cli.PExpire(ctx, key, time.Duration(ttlMs)*time.Millisecond).Err(); e != nil {
			return e
		}
	}
	m.auditKey(b, "key_edit", "CREATE", key, typ)
	return nil
}

// auditKey 记录键操作审计。
func (m *Manager) auditKey(b *Bundle, category, action, target, detail string) {
	m.audit.Log(audit.Entry{
		ConnID: b.ID, ConnName: b.Cfg.Name, Category: category,
		Action: action, Target: target, Detail: detail, Result: "ok", Level: "info",
	})
}

func rawValue(v string, binary bool) any {
	if binary {
		raw, err := base64.StdEncoding.DecodeString(v)
		if err == nil {
			return raw
		}
	}
	return v
}
