package rds

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"RedisLens/internal/audit"
	"RedisLens/internal/config"
)

// 测试主入口：优先使用真实 Redis 实例（DUMP/RESTORE/SLOWLOG/INFO 分区完整），
// 不可用时回退 miniredis（部分能力受限的用例自动跳过）。
var testRealAddr string // 真实实例地址；空表示 miniredis

func TestMain(m *testing.M) {
	addr := os.Getenv("REDISLENS_TEST_ADDR")
	if addr == "" {
		for _, cand := range []string{
			`D:/gitrepo/RedisLens-tools/redis/redis-server.exe`,
			"redis-server",
		} {
			if _, err := os.Stat(cand); err == nil {
				addr = startRedisServer(cand)
				break
			}
		}
	}
	if addr != "" {
		cli := redis.NewClient(&redis.Options{Addr: addr, DialTimeout: 2 * time.Second})
		if err := cli.Ping(context.Background()).Err(); err == nil {
			_ = cli.FlushAll(context.Background()).Err()
			_ = cli.Close()
			testRealAddr = addr
		}
	}
	code := m.Run()
	if redisServerProc != nil {
		_ = redisServerProc.Process.Kill()
	}
	os.Exit(code)
}

var redisServerProc *exec.Cmd

// startRedisServer 使用随机空闲端口启动测试实例并等待就绪，
// 避免与残留实例/其他进程的端口竞争（修复偶发的测试 flakiness）。
func startRedisServer(exe string) string {
	dir, _ := os.MkdirTemp("", "redislens-test")
	for attempt := 0; attempt < 5; attempt++ {
		port := 20000 + int(time.Now().UnixNano()%20000) + attempt
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		cmd := exec.Command(exe, "--port", strconv.Itoa(port), "--save", "", "--dir", dir, "--daemonize", "no")
		if err := cmd.Start(); err != nil {
			continue
		}
		// 等待就绪（最多 3 秒）
		ready := false
		for i := 0; i < 30; i++ {
			cli := redis.NewClient(&redis.Options{Addr: addr, DialTimeout: 200 * time.Millisecond})
			err := cli.Ping(context.Background()).Err()
			_ = cli.Close()
			if err == nil {
				ready = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if ready {
			redisServerProc = cmd
			return addr
		}
		_ = cmd.Process.Kill()
	}
	return ""
}

// testEnv redis + manager 测试环境。
type testEnv struct {
	mr   *miniredis.Miniredis
	mgr  *Manager
	st   *config.Store
	al   *audit.Logger
	Conn *config.Connection
	cli  redis.UniversalClient
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	var mr *miniredis.Miniredis
	var addr string
	if testRealAddr != "" {
		addr = testRealAddr
	} else {
		mr = miniredis.RunT(t)
		mr.FlushAll()
		addr = mr.Addr()
	}

	dir := t.TempDir()
	st, err := config.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	al := audit.New(filepath.Join(dir, "audit"), true)
	mgr := NewManager(st, al, func(string, any) {})
	conn := &config.Connection{
		ID:   "test-conn",
		Name: "unit-test",
		Mode: "standalone",
		Host: "127.0.0.1",
		Port: parsePort(addr),
		DB:   0,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := mgr.Open(ctx, conn); err != nil {
		t.Fatalf("open connection: %v", err)
	}
	cli := redis.NewClient(&redis.Options{Addr: addr})
	_ = cli.FlushAll(context.Background()).Err() // 每个用例独立数据集
	return &testEnv{mr: mr, mgr: mgr, st: st, al: al, Conn: conn, cli: cli}
}

func (e *testEnv) Close() {
	_ = e.cli.Close()
	e.mgr.CloseAll()
	e.al.Close()
}

func parsePort(addr string) int {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	p, _ := strconv.Atoi(portStr)
	return p
}

// ---------- F-02: SCAN 分页与树 ----------

func TestScanPageAndTree(t *testing.T) {
	env := newTestEnv(t)
	defer env.Close()

	// 预置 250 个键 + 多类型
	for i := 0; i < 250; i++ {
		env.cli.Set(ctx(), "user:profile:"+itoa(i), "value", 0)
	}
	env.cli.HSet(ctx(), "hash:1", "f1", "v1", "f2", "v2")
	env.cli.Set(ctx(), "plain", "x", 0)

	resp, err := env.mgr.ScanPage("test-conn", 0, 0, "user:*", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Keys) == 0 {
		t.Fatal("expected keys from scan")
	}
	if resp.Keys[0].Type != "string" {
		t.Errorf("type should be string, got %s", resp.Keys[0].Type)
	}
	// 分页遍历完
	total := len(resp.Keys)
	cursor := resp.Cursor
	for cursor != 0 {
		r, err := env.mgr.ScanPage("test-conn", 0, cursor, "user:*", "", 100)
		if err != nil {
			t.Fatal(err)
		}
		total += len(r.Keys)
		cursor = r.Cursor
	}
	if total != 250 {
		t.Errorf("expected 250 keys total, got %d", total)
	}

	// 类型过滤
	resp, err = env.mgr.ScanPage("test-conn", 0, 0, "*", "hash", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Keys) != 1 || resp.Keys[0].Key != "hash:1" {
		t.Errorf("type filter failed: %+v", resp.Keys)
	}

	// 树
	nodes, err := env.mgr.KeyTree("test-conn", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	var hasFolder, hasLeaf bool
	for _, n := range nodes {
		if n.IsFolder && n.Name == "user" {
			hasFolder = true
		}
		if !n.IsFolder && n.Name == "plain" {
			hasLeaf = true
		}
	}
	if !hasFolder || !hasLeaf {
		t.Errorf("tree missing folder/leaf: %+v", nodes)
	}

	// 子层级
	sub, err := env.mgr.KeyTree("test-conn", 0, "user")
	if err != nil {
		t.Fatal(err)
	}
	var hasProfile bool
	for _, n := range sub {
		if n.IsFolder && n.Name == "profile" {
			hasProfile = true
		}
	}
	if !hasProfile {
		t.Errorf("sub-tree missing profile folder: %+v", sub)
	}
}

// ---------- F-03: 各类型值查看与编辑 ----------

func TestValueViewAndEdit(t *testing.T) {
	env := newTestEnv(t)
	defer env.Close()

	// string + JSON
	env.cli.Set(ctx(), "s:1", `{"name":"RedisLens","n":42}`, 0)
	vd, err := env.mgr.GetValue("test-conn", 0, "s:1")
	if err != nil {
		t.Fatal(err)
	}
	if !vd.IsJSON {
		t.Error("JSON detection failed")
	}
	if err := env.mgr.SetString("test-conn", 0, "s:1", `{"name":"updated"}`, false, -1); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.cli.Get(ctx(), "s:1").Result(); got != `{"name":"updated"}` {
		t.Errorf("set string failed: %s", got)
	}

	// hash 编辑
	env.cli.HSet(ctx(), "h:1", "f1", "v1")
	if err := env.mgr.HashSetField("test-conn", 0, "h:1", "f1", "v1-updated", false); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.cli.HGet(ctx(), "h:1", "f1").Result(); got != "v1-updated" {
		t.Errorf("hset failed: %s", got)
	}
	if err := env.mgr.HashDelFields("test-conn", 0, "h:1", []string{"f1"}); err != nil {
		t.Fatal(err)
	}
	if n, _ := env.cli.HLen(ctx(), "h:1").Result(); n != 0 {
		t.Errorf("hdel failed: %d", n)
	}

	// list 编辑（LSET / 推入 / 哨兵删除）
	env.cli.RPush(ctx(), "l:1", "a", "b", "c")
	if err := env.mgr.ListSetIndex("test-conn", 0, "l:1", 1, "B!", false); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.cli.LIndex(ctx(), "l:1", 1).Result(); got != "B!" {
		t.Errorf("lset failed: %s", got)
	}
	if err := env.mgr.ListPush("test-conn", 0, "l:1", "left", "HEAD", false); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.cli.LIndex(ctx(), "l:1", 0).Result(); got != "HEAD" {
		t.Errorf("lpush failed: %s", got)
	}
	if err := env.mgr.ListRemoveIndex("test-conn", 0, "l:1", 0); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.cli.LIndex(ctx(), "l:1", 0).Result(); got != "a" {
		t.Errorf("lrem-by-index failed: %s", got)
	}

	// set
	if err := env.mgr.SetAddMembers("test-conn", 0, "s:set", []string{"m1", "m2"}); err != nil {
		t.Fatal(err)
	}
	if err := env.mgr.SetRemoveMembers("test-conn", 0, "s:set", []string{"m1"}); err != nil {
		t.Fatal(err)
	}
	if n, _ := env.cli.SCard(ctx(), "s:set").Result(); n != 1 {
		t.Errorf("set ops failed: %d", n)
	}

	// zset
	if err := env.mgr.ZSetUpsert("test-conn", 0, "z:1", "member1", 3.14, false); err != nil {
		t.Fatal(err)
	}
	score, _ := env.cli.ZScore(ctx(), "z:1", "member1").Result()
	if score != 3.14 {
		t.Errorf("zadd failed: %v", score)
	}
	if err := env.mgr.ZSetUpsert("test-conn", 0, "z:1", "member1", 9.9, false); err != nil {
		t.Fatal(err)
	}
	score, _ = env.cli.ZScore(ctx(), "z:1", "member1").Result()
	if score != 9.9 {
		t.Errorf("zadd update failed: %v", score)
	}
	if err := env.mgr.ZSetRemoveMembers("test-conn", 0, "z:1", []string{"member1"}); err != nil {
		t.Fatal(err)
	}
	if n, _ := env.cli.ZCard(ctx(), "z:1").Result(); n != 0 {
		t.Error("zrem failed")
	}

	// stream
	if err := env.mgr.StreamAdd("test-conn", 0, "st:1", "*", map[string]string{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	entries, _ := env.cli.XRange(ctx(), "st:1", "-", "+").Result()
	if len(entries) != 1 {
		t.Fatal("xadd failed")
	}
	if err := env.mgr.StreamDelete("test-conn", 0, "st:1", []string{entries[0].ID}); err != nil {
		t.Fatal(err)
	}
	if n, _ := env.cli.XLen(ctx(), "st:1").Result(); n != 0 {
		t.Error("xdel failed")
	}

	// 创建键
	if err := env.mgr.CreateKey("test-conn", 0, "new:list", "list", "hello", -2); err != nil {
		t.Fatal(err)
	}
	if got, _ := env.cli.LRange(ctx(), "new:list", 0, -1).Result(); len(got) != 1 || got[0] != "hello" {
		t.Errorf("create list failed: %v", got)
	}
}

// skipIfMiniredis 依赖 DUMP/RESTORE/SLOWLOG 等真实命令时使用。
func skipIfMiniredis(t *testing.T) {
	t.Helper()
	if testRealAddr == "" {
		t.Skip("miniredis 不支持 DUMP/RESTORE/SLOWLOG 等命令，真实实例下验证")
	}
}

// ---------- F-04: TTL ----------

func TestTTLAndRecycle(t *testing.T) {
	skipIfMiniredis(t)
	env := newTestEnv(t)
	defer env.Close()

	env.cli.Set(ctx(), "k:1", "v1", 0)
	env.cli.Set(ctx(), "k:2", "v2", 0)

	// 批量设置 TTL
	if _, err := env.mgr.SetTTL("test-conn", 0, []string{"k:1", "k:2"}, 60000); err != nil {
		t.Fatal(err)
	}
	if n, _ := env.cli.Exists(ctx(), "k:1").Result(); n == 0 {
		t.Fatal("key should exist")
	}
	ttl, _ := env.cli.TTL(ctx(), "k:1").Result()
	if ttl <= 0 {
		t.Errorf("ttl should be positive, got %v", ttl)
	}

	// 移除 TTL
	if _, err := env.mgr.SetTTL("test-conn", 0, []string{"k:1"}, -2); err != nil {
		t.Fatal(err)
	}
	ttl, _ = env.cli.TTL(ctx(), "k:1").Result()
	if ttl != -1 {
		t.Errorf("persist failed: %v", ttl)
	}

	// 删除 + 回收站恢复（F-07）
	if res, err := env.mgr.DeleteKeys("test-conn", 0, []string{"k:2"}, true); err != nil {
		t.Fatal(err)
	} else {
		if res.Deleted != 1 || res.BackedUp != 1 {
			t.Errorf("delete result wrong: %+v", res)
		}
	}
	if n, _ := env.cli.Exists(ctx(), "k:2").Result(); n > 0 {
		t.Error("k:2 should be deleted")
	}
	items, _ := env.mgr.RecycleList("test-conn")
	if len(items) != 1 || items[0].Key != "k:2" {
		t.Fatalf("recycle bin wrong: %+v", items)
	}
	if err := env.mgr.RecycleRestore("test-conn", items[0].Token); err != nil {
		t.Fatal(err)
	}
	if n, _ := env.cli.Exists(ctx(), "k:2").Result(); n == 0 {
		t.Error("k:2 should be restored")
	}
	if got, _ := env.cli.Get(ctx(), "k:2").Result(); got != "v2" {
		t.Errorf("restored value wrong: %s", got)
	}

	// SET 后保留 TTL
	env.cli.Set(ctx(), "k:3", "old", 0)
	if _, err := env.mgr.SetTTL("test-conn", 0, []string{"k:3"}, 120000); err != nil {
		t.Fatal(err)
	}
	if err := env.mgr.SetString("test-conn", 0, "k:3", "new", false, -1); err != nil {
		t.Fatal(err)
	}
	pttl, _ := env.cli.PTTL(ctx(), "k:3").Result()
	if pttl <= 0 {
		t.Errorf("keep-ttl failed: %v", pttl)
	}
}

// ---------- F-06: 控制台危险命令拦截 ----------

func TestConsoleGuard(t *testing.T) {
	skipIfMiniredis(t)
	env := newTestEnv(t)
	defer env.Close()

	// 正常命令
	res, err := env.mgr.ExecCommand("test-conn", "SET hello world", false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Blocked {
		t.Errorf("set should pass: %+v", res)
	}

	// 危险命令拦截
	res, err = env.mgr.ExecCommand("test-conn", "FLUSHALL", false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Blocked || !res.NeedConf {
		t.Errorf("FLUSHALL should be blocked: %+v", res)
	}
	// 未确认执行不应生效
	res, err = env.mgr.ExecCommand("test-conn", "CONFIG SET maxmemory 100mb", false)
	if err != nil || !res.Blocked {
		t.Errorf("CONFIG SET should be blocked: %+v err=%v", res, err)
	}

	// 确认后放行并审计
	res, err = env.mgr.ExecCommand("test-conn", "FLUSHDB", true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Blocked {
		t.Errorf("confirmed FLUSHDB should pass: %+v", res)
	}

	// 引号参数
	res, err = env.mgr.ExecCommand("test-conn", `SET "quoted key" "some value"`, false)
	if err != nil || !res.OK {
		t.Errorf("quoted set failed: %+v err=%v", res, err)
	}
	if got, _ := env.cli.Get(ctx(), "quoted key").Result(); got != "some value" {
		t.Errorf("quoted set value wrong: %q", got)
	}

	// 错误命令
	res, err = env.mgr.ExecCommand("test-conn", "NOSUCHCMD", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Error("unknown command should fail")
	}

	// 审计日志留痕
	entries, _ := env.al.Query(50)
	var hasDangerBlocked, hasDangerConfirmed bool
	for _, e := range entries {
		if e.Category == "dangerous" && e.Result == "blocked" {
			hasDangerBlocked = true
		}
		if e.Category == "dangerous" && e.Result == "ok" {
			hasDangerConfirmed = true
		}
	}
	if !hasDangerBlocked || !hasDangerConfirmed {
		t.Errorf("audit entries missing: blocked=%v confirmed=%v", hasDangerBlocked, hasDangerConfirmed)
	}
}

// 只读模式拦截（F-01 一键只读）
func TestReadOnlyEnforcement(t *testing.T) {
	env := newTestEnv(t)
	defer env.Close()

	// 只读模式直接作用在活动 Bundle 上（对应 UI 一键只读）
	b, err := env.mgr.Get("test-conn")
	if err != nil {
		t.Fatal(err)
	}
	b.ReadOnly = true

	// 读放行
	if _, err := env.mgr.ExecCommand("test-conn", "GET foo", false); err != nil {
		t.Fatal(err)
	}
	// 写拦截（普通写）
	res, err := env.mgr.ExecCommand("test-conn", "SET foo bar", false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Blocked || res.NeedConf {
		t.Errorf("write should be blocked in readonly: %+v", res)
	}
	// 即使确认也不能绕过只读
	res, err = env.mgr.ExecCommand("test-conn", "SET foo bar", true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Blocked {
		t.Error("readonly cannot be bypassed by confirm")
	}
	// 删除也拦截
	if _, err := env.mgr.DeleteKeys("test-conn", 0, []string{"x"}, false); err == nil {
		t.Error("DeleteKeys should fail in readonly")
	}
}

// ---------- F-05: 监控采样 ----------

func TestMonitorSampling(t *testing.T) {
	skipIfMiniredis(t)
	env := newTestEnv(t)
	defer env.Close()

	m := NewMonitor("test-conn", env.mgr.bundles["test-conn"].Client, 100*time.Millisecond, func(string, any) {})
	m.Start()
	defer m.Stop()
	time.Sleep(350 * time.Millisecond)
	hist := m.History(3600)
	if len(hist) < 2 {
		t.Fatalf("expected at least 2 samples, got %d", len(hist))
	}
	s := hist[len(hist)-1]
	if s.Version == "" {
		t.Error("version should be set from INFO")
	}
	if s.TotalKeys < 0 {
		t.Error("totalKeys should be >= 0")
	}
}

// ---------- F-09: 大 Key 扫描 / 慢查询 ----------

func TestBigKeyScan(t *testing.T) {
	env := newTestEnv(t)
	defer env.Close()

	// 预置数据
	for i := 0; i < 50; i++ {
		env.cli.Set(ctx(), "bk:"+itoa(i), "value-"+itoa(i), 0)
	}
	env.cli.RPush(ctx(), "big:list", "x", "y", "z")

	var wg sync.WaitGroup
	var report *BigKeyReport
	wg.Add(1)
	env.mgr.emit = func(event string, payload any) {
		if event == "analysis:done" {
			report = payload.(map[string]any)["report"].(*BigKeyReport)
			wg.Done()
		}
	}
	taskID, err := env.mgr.BeginBigKeyScan("test-conn", 10, 100000)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		env.mgr.StopBigKeyScan(taskID)
		t.Fatal("bigkey scan timeout")
	}
	if report == nil {
		t.Fatal("no report")
	}
	if report.ScannedKeys != 51 {
		t.Errorf("expected 51 scanned, got %d", report.ScannedKeys)
	}
	if len(report.TypeDist) == 0 {
		t.Error("type distribution empty")
	}
	if len(report.TTLBuckets) == 0 {
		t.Error("ttl buckets empty")
	}
}

func TestSlowLogAndCmdStats(t *testing.T) {
	skipIfMiniredis(t)
	env := newTestEnv(t)
	defer env.Close()

	entries, err := env.mgr.GetSlowLog("test-conn", 10)
	if err != nil {
		t.Fatal(err)
	}
	// 可能为空，但不应报错
	_ = entries

	stats, err := env.mgr.GetCommandStats("test-conn")
	if err != nil {
		t.Fatal(err)
	}
	_ = stats
}

// ---------- F-10: 导入导出 ----------

func TestImportExportRoundTrip(t *testing.T) {
	skipIfMiniredis(t)
	env := newTestEnv(t)
	defer env.Close()

	// 预置
	env.cli.Set(ctx(), "exp:str", "hello world", 0)
	env.cli.HSet(ctx(), "exp:hash", "f", "v")
	env.cli.RPush(ctx(), "exp:list", "a", "b")
	env.cli.SAdd(ctx(), "exp:set", "m")
	env.cli.ZAdd(ctx(), "exp:zset", redis.Z{Score: 1.5, Member: "z"})
	if err := env.cli.Expire(ctx(), "exp:str", 300*time.Second).Err(); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(t.TempDir(), "export.json")
	report, err := env.mgr.ExportKeys("test-conn", 0, "exp:*", "", file)
	if err != nil {
		t.Fatal(err)
	}
	if report.Keys != 5 {
		t.Errorf("expected 5 keys exported, got %d", report.Keys)
	}

	// 清空后导入
	env.cli.FlushAll(ctx())
	imp, err := env.mgr.ImportFile("test-conn", 0, file, true)
	if err != nil {
		t.Fatal(err)
	}
	if imp.Imported != 5 {
		t.Errorf("expected 5 imported, got %d (failed %d)", imp.Imported, imp.Failed)
	}
	// 校验数据完整性
	if got, _ := env.cli.Get(ctx(), "exp:str").Result(); got != "hello world" {
		t.Errorf("string restore failed: %q", got)
	}
	if n, _ := env.cli.HLen(ctx(), "exp:hash").Result(); n != 1 {
		t.Error("hash restore failed")
	}
	if n, _ := env.cli.LLen(ctx(), "exp:list").Result(); n != 2 {
		t.Error("list restore failed")
	}
	if n, _ := env.cli.SCard(ctx(), "exp:set").Result(); n != 1 {
		t.Error("set restore failed")
	}
	if n, _ := env.cli.ZCard(ctx(), "exp:zset").Result(); n != 1 {
		t.Error("zset restore failed")
	}
	// TTL 恢复
	ttl, _ := env.cli.TTL(ctx(), "exp:str").Result()
	if ttl <= 0 {
		t.Errorf("ttl restore failed: %v", ttl)
	}
}

func TestPreviewDelete(t *testing.T) {
	env := newTestEnv(t)
	defer env.Close()
	for i := 0; i < 20; i++ {
		env.cli.Set(ctx(), "del:me:"+itoa(i), "x", 0)
	}
	env.cli.Set(ctx(), "keep:me", "x", 0)
	pv, err := env.mgr.PreviewDelete("test-conn", 0, "del:me:*", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if pv.Total != 20 {
		t.Errorf("expected 20, got %d", pv.Total)
	}
	if len(pv.Samples) != 20 {
		t.Errorf("expected 20 samples, got %d", len(pv.Samples))
	}
}

// ---------- F-11: 发布订阅 ----------

func TestPubSub(t *testing.T) {
	env := newTestEnv(t)
	defer env.Close()
	_ = env.mr

	var mu sync.Mutex
	var got []PubSubMessage
	env.mgr.emit = func(event string, payload any) {
		if event == "pubsub:message" {
			mu.Lock()
			got = append(got, payload.(PubSubMessage))
			mu.Unlock()
		}
	}

	// 订阅
	if err := env.mgr.PubSubSubscribe("test-conn", []string{"ch:1"}, nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // 等待订阅生效

	// 发布
	n, err := env.mgr.Publish("test-conn", "ch:1", "hello-pubsub")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 receiver, got %d", n)
	}
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(got) == 0 {
		t.Fatal("no pubsub message received")
	}
	var hasPayload bool
	for _, m := range got {
		if m.Channel == "ch:1" && m.Payload == "hello-pubsub" && m.Kind == "message" {
			hasPayload = true
		}
	}
	if !hasPayload {
		t.Errorf("message mismatch: %+v", got)
	}

	// 退订
	if err := env.mgr.PubSubUnsubscribe("test-conn"); err != nil {
		t.Fatal(err)
	}
}

// ---------- F-08: 拓扑（standalone） ----------

func TestTopologyStandalone(t *testing.T) {
	skipIfMiniredis(t)
	env := newTestEnv(t)
	defer env.Close()

	topo, err := env.mgr.GetTopology("test-conn")
	if err != nil {
		t.Fatal(err)
	}
	if topo.Mode != "standalone" {
		t.Errorf("mode = %s", topo.Mode)
	}
	if len(topo.Nodes) == 0 {
		t.Error("no nodes")
	}
}

// ---------- 审计日志 ----------

func TestAuditLog(t *testing.T) {
	dir := t.TempDir()
	al := audit.New(dir, true)
	defer al.Close()

	al.Log(audit.Entry{Category: "command", Action: "GET", Target: "foo", Result: "ok", Level: "info"})
	al.Log(audit.Entry{Category: "dangerous", Action: "FLUSHALL", Target: "FLUSHALL", Result: "blocked", Level: "warn"})

	entries, err := al.Query(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Category != "dangerous" {
		t.Errorf("query should be newest first: %+v", entries[0])
	}
	_ = os.Stdout
}

// ctx 返回短超时上下文。
func ctx() context.Context {
	c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = cancel // 测试生命周期内不取消
	return c
}

// 工具函数
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
