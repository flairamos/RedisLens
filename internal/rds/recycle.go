package rds

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RecycleItem 回收站条目：被删除键的完整备份。
type RecycleItem struct {
	Token     string    `json:"token"`
	Key       string    `json:"key"`
	Type      string    `json:"type"`
	PTTLms    int64     `json:"pttl"`
	Dump      []byte    `json:"-"`
	DeletedAt time.Time `json:"deletedAt"`
	Size      int       `json:"size"`
}

// RecycleBin 内存回收站：删除前备份（DUMP），支持一键恢复。
// 容量受 maxItems 与 maxBytes 双限制，超出时丢弃最旧条目。
type RecycleBin struct {
	mu       sync.Mutex
	items    []*RecycleItem
	maxItems int
	maxBytes int64
	bytes    int64
	seq      int
}

// NewRecycleBin 创建回收站（默认 100 条 / 64MB）。
func NewRecycleBin() *RecycleBin {
	return &RecycleBin{maxItems: 100, maxBytes: 64 << 20}
}

// Put 备份并登记待删除键，返回回收站 token。
func (r *RecycleBin) Put(ctx context.Context, client redis.UniversalClient, key, typ string) (string, error) {
	dump, err := client.Dump(ctx, key).Bytes()
	if err != nil {
		return "", fmt.Errorf("dump %s: %w", key, err)
	}
	pttl, err := client.PTTL(ctx, key).Result()
	if err != nil {
		pttl = -1
	}
	pttlMs := ttlToMs(pttl)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	item := &RecycleItem{
		Key:       key,
		Type:      typ,
		PTTLms:    pttlMs,
		Dump:      dump,
		DeletedAt: time.Now(),
		Size:      len(dump),
		Token:     fmt.Sprintf("rb-%d-%d", time.Now().UnixNano(), r.seq),
	}
	r.items = append(r.items, item)
	r.bytes += int64(len(dump))
	r.evictLocked()
	return item.Token, nil
}

// List 返回回收站条目（时间倒序）。
func (r *RecycleBin) List() []RecycleItem {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]RecycleItem, 0, len(r.items))
	for i := len(r.items) - 1; i >= 0; i-- {
		it := r.items[i]
		out = append(out, RecycleItem{
			Token: it.Token, Key: it.Key, Type: it.Type,
			PTTLms: it.PTTLms, DeletedAt: it.DeletedAt, Size: it.Size,
		})
	}
	return out
}

// Restore 按 token 恢复键（RESTORE REPLACE，保留原 TTL）。
func (r *RecycleBin) Restore(ctx context.Context, client redis.UniversalClient, token string) error {
	r.mu.Lock()
	var item *RecycleItem
	for _, it := range r.items {
		if it.Token == token {
			item = it
			break
		}
	}
	r.mu.Unlock()
	if item == nil {
		return fmt.Errorf("recycle item %s not found", token)
	}
	// RESTORE 的 ttl 必须 >= 0：永久键（PTTLms == -1）传 0 表示无过期
	ttl := time.Duration(item.PTTLms) * time.Millisecond
	if item.PTTLms < 0 {
		ttl = 0
	}
	return client.RestoreReplace(ctx, item.Key, ttl, string(item.Dump)).Err()
}

// Remove 移除回收站条目（彻底删除）。
func (r *RecycleBin) Remove(token string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.items[:0]
	for _, it := range r.items {
		if it.Token == token {
			r.bytes -= int64(it.Size)
			continue
		}
		kept = append(kept, it)
	}
	r.items = kept
}

// Clear 清空回收站。
func (r *RecycleBin) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = nil
	r.bytes = 0
}

// Stats 回收站统计。
func (r *RecycleBin) Stats() (items int, bytes int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.items), r.bytes
}

// evictLocked 超限时按最旧淘汰。调用方需持有锁。
func (r *RecycleBin) evictLocked() {
	for len(r.items) > r.maxItems || r.bytes > int64(r.maxBytes) {
		if len(r.items) == 0 {
			return
		}
		oldest := r.items[0]
		r.bytes -= int64(oldest.Size)
		r.items = r.items[1:]
	}
}
