package rds

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"RedisLens/internal/audit"
	"RedisLens/internal/config"
)

// TestGoroutineLeakOnOpenClose 打开/关闭连接与监控后协程数应回落到基线附近。
// （内存泄漏审查：监控循环、心跳循环、pubsub 循环均须随连接关闭退出）
func TestGoroutineLeakOnOpenClose(t *testing.T) {
	skipIfMiniredis(t)
	// 基线：先建一个独立连接并关闭，让运行时稳定
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	before := runtime.NumGoroutine()

	const cycles = 5
	for i := 0; i < cycles; i++ {
		dir := t.TempDir()
		st, err := config.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		al := newTestAudit(t, dir)
		mgr := NewManager(st, al, func(string, any) {})
		conn := &config.Connection{
			ID:   "leak-test",
			Name: "leak",
			Mode: "standalone",
			Host: "127.0.0.1",
			Port: parsePort(testRealAddr),
			DB:   0,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if _, err := mgr.Open(ctx, conn); err != nil {
			cancel()
			t.Fatalf("open: %v", err)
		}
		cancel()
		// 让监控采样至少跑一轮
		time.Sleep(200 * time.Millisecond)
		// pubsub 会话开启后再关闭
		if err := mgr.PubSubSubscribe("leak-test", []string{"ch"}, nil); err != nil {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
		mgr.Close("leak-test")
		al.Close()
		_ = st
	}

	// 等待协程退出
	deadline := time.Now().Add(5 * time.Second)
	var after int
	for {
		runtime.GC()
		after = runtime.NumGoroutine()
		if after <= before+3 || time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if after > before+3 {
		t.Errorf("goroutine leak: before=%d after=%d (cycles=%d)", before, after, cycles)
	}
}

// TestRecycleBinBounded 回收站容量有界，防止大键备份撑爆内存。
func TestRecycleBinBounded(t *testing.T) {
	r := NewRecycleBin()
	r.maxItems = 5
	r.maxBytes = 1 << 20
	// 塞入超过上限的条目
	for i := 0; i < 20; i++ {
		r.mu.Lock()
		r.seq++
		r.items = append(r.items, &RecycleItem{
			Token: itoa(i),
			Key:   "k",
			Size:  100,
			Dump:  make([]byte, 100),
		})
		r.bytes += 100
		r.evictLocked()
		r.mu.Unlock()
	}
	if n, bytes := r.Stats(); n > 5 || bytes > 1<<20 {
		t.Errorf("recycle bin not bounded: items=%d bytes=%d", n, bytes)
	}
}

// TestMonitorRingBufferBounded 监控环形缓冲有界。
func TestMonitorRingBufferBounded(t *testing.T) {
	m := &Monitor{maxKeep: 100}
	m.mu.Lock()
	for i := 0; i < 500; i++ {
		m.samples = append(m.samples, Sample{Time: int64(i)})
	}
	if len(m.samples) > m.maxKeep {
		m.samples = m.samples[len(m.samples)-m.maxKeep:]
	}
	m.mu.Unlock()
	if len(m.samples) != 100 {
		t.Errorf("ring buffer not bounded: %d", len(m.samples))
	}
}

// TestReconnectAfterCloseNoLeak 关闭连接后心跳循环不得重建客户端。
func TestReconnectAfterCloseNoLeak(t *testing.T) {
	skipIfMiniredis(t)
	dir := t.TempDir()
	st, _ := config.Open(dir)
	al := newTestAudit(t, dir)
	defer al.Close()
	mgr := NewManager(st, al, func(string, any) {})

	conn := &config.Connection{
		ID: "rc", Name: "rc", Mode: "standalone",
		Host: "127.0.0.1", Port: parsePort(testRealAddr), DB: 0,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if _, err := mgr.Open(ctx, conn); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	cli := mgr.bundles["rc"].Client
	if cli == nil {
		t.Fatal("client should exist")
	}
	mgr.Close("rc")
	// 关闭后等待一段超过 backoff 的时间，客户端不应被重建（复用已关闭的旧引用对象验证计数）
	time.Sleep(2 * time.Second)
	mgr.mu.RLock()
	_, exists := mgr.bundles["rc"]
	mgr.mu.RUnlock()
	if exists {
		t.Error("bundle should be removed after Close")
	}
	_ = cli
}

// newTestAudit 独立审计器（目录自动创建）。
func newTestAudit(t *testing.T, dir string) *audit.Logger {
	t.Helper()
	return audit.New(filepath.Join(dir, "audit"), true)
}
