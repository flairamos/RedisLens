package rds

import (
	"fmt"
	"strings"
	"time"

	"RedisLens/internal/audit"
)

// DeleteResult 删除结果。
type DeleteResult struct {
	Deleted  int      `json:"deleted"`
	BackedUp int      `json:"backedUp"`
	Tokens   []string `json:"tokens"` // 回收站条目 token（每个键一个）
}

// DeleteKeys 删除键。backup=true 时逐键 DUMP 进回收站（支持撤销），再 UNLINK。
func (m *Manager) DeleteKeys(connID string, db int, keys []string, backup bool) (*DeleteResult, error) {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return nil, err
	}
	if err := m.assertWritable(b, "DEL"); err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return &DeleteResult{}, nil
	}
	ctx, cancel := opCtx()
	defer cancel()

	res := &DeleteResult{}
	for _, k := range keys {
		typ, err := cli.Type(ctx, k).Result()
		if err != nil || typ == "none" {
			continue
		}
		typ = normalizeType(typ)
		if backup {
			token, err := b.Recycle.Put(ctx, cli, k, typ)
			if err != nil {
				return res, fmt.Errorf("备份键 %s 失败，已中止删除: %w", k, err)
			}
			res.Tokens = append(res.Tokens, token)
			res.BackedUp++
		}
		if err := cli.Unlink(ctx, k).Err(); err != nil {
			return res, err
		}
		res.Deleted++
	}
	m.audit.Log(audit.Entry{
		ConnID: b.ID, ConnName: b.Cfg.Name, Category: "key_delete",
		Action: "DELETE", Target: fmt.Sprintf("%d keys", res.Deleted),
		Detail: summarizeKeys(keys), Result: "ok", Level: "warn",
	})
	return res, nil
}

func summarizeKeys(keys []string) string {
	if len(keys) <= 5 {
		return strings.Join(keys, ", ")
	}
	return strings.Join(keys[:5], ", ") + fmt.Sprintf(" ...(%d total)", len(keys))
}

// RenameKey 重命名键。
func (m *Manager) RenameKey(connID string, db int, oldKey, newKey string) error {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "RENAME"); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.Rename(ctx, oldKey, newKey).Err(); err != nil {
		return err
	}
	m.audit.Log(audit.Entry{
		ConnID: b.ID, ConnName: b.Cfg.Name, Category: "key_edit",
		Action: "RENAME", Target: oldKey, Detail: "-> " + newKey, Result: "ok", Level: "info",
	})
	return nil
}

// TTLResult TTL 批量结果。
type TTLResult struct {
	Affected int `json:"affected"`
}

// SetTTL 批量设置 TTL。ttlMs: >=0 设置；-2 移除（PERSIST）。
func (m *Manager) SetTTL(connID string, db int, keys []string, ttlMs int64) (*TTLResult, error) {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return nil, err
	}
	if err := m.assertWritable(b, "EXPIRE/PERSIST"); err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	res := &TTLResult{}
	for _, k := range keys {
		var e error
		if ttlMs == -2 {
			e = cli.Persist(ctx, k).Err()
		} else {
			e = cli.PExpire(ctx, k, time.Duration(ttlMs)*time.Millisecond).Err()
		}
		if e == nil {
			res.Affected++
		}
	}
	level := "info"
	if ttlMs == -2 {
		level = "warn"
	}
	m.audit.Log(audit.Entry{
		ConnID: b.ID, ConnName: b.Cfg.Name, Category: "key_edit",
		Action: map[bool]string{true: "PERSIST", false: "EXPIRE"}[ttlMs == -2],
		Target: fmt.Sprintf("%d keys", res.Affected), Detail: summarizeKeys(keys),
		Result: "ok", Level: level,
	})
	return res, nil
}

// RecycleList 回收站条目（含连接过滤）。
func (m *Manager) RecycleList(connID string) ([]RecycleItem, error) {
	b, err := m.Get(connID)
	if err != nil {
		return nil, err
	}
	return b.Recycle.List(), nil
}

// RecycleRestore 从回收站恢复键。
func (m *Manager) RecycleRestore(connID, token string) error {
	b, err := m.Get(connID)
	if err != nil {
		return err
	}
	if err := m.assertWritable(b, "RESTORE"); err != nil {
		return err
	}
	cli, _, err := m.dbClient(connID, b.Cfg.DB)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := b.Recycle.Restore(ctx, cli, token); err != nil {
		return err
	}
	m.audit.Log(audit.Entry{
		ConnID: b.ID, ConnName: b.Cfg.Name, Category: "key_delete",
		Action: "RESTORE", Target: token, Result: "ok", Level: "info",
	})
	return nil
}

// RecycleRemove 彻底移除回收站条目。
func (m *Manager) RecycleRemove(connID, token string) error {
	b, err := m.Get(connID)
	if err != nil {
		return err
	}
	b.Recycle.Remove(token)
	return nil
}

// RecycleClear 清空回收站。
func (m *Manager) RecycleClear(connID string) error {
	b, err := m.Get(connID)
	if err != nil {
		return err
	}
	b.Recycle.Clear()
	return nil
}

// ScanStreamPage 集合类型分页读取（hash/set 用 SCAN 子命令，zset/list 用 rank 分页）。
func (m *Manager) ScanStreamPage(connID string, db int, key, typ string, cursor uint64, match string, count int64) (cursorOut uint64, total int64, items []FieldItem, err error) {
	cli, _, err := m.dbClient(connID, db)
	if err != nil {
		return 0, 0, nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	switch typ {
	case "hash":
		total, err = cli.HLen(ctx, key).Result()
		if err != nil {
			return
		}
		var flat []string
		flat, cursorOut, err = cli.HScan(ctx, key, cursor, match, count).Result()
		if err != nil {
			return
		}
		for i := 0; i+1 < len(flat); i += 2 {
			text, binary := bytesToVal([]byte(flat[i+1]))
			items = append(items, FieldItem{Field: flat[i], Value: text, Binary: binary})
		}
	case "set":
		total, err = cli.SCard(ctx, key).Result()
		if err != nil {
			return
		}
		var vals []string
		vals, cursorOut, err = cli.SScan(ctx, key, cursor, match, count).Result()
		if err != nil {
			return
		}
		for _, v := range vals {
			text, binary := bytesToVal([]byte(v))
			items = append(items, FieldItem{Field: text, Value: "", Binary: binary})
		}
	default:
		err = fmt.Errorf("type %s does not support cursor paging", typ)
	}
	return
}
