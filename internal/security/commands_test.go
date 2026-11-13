package security

import (
	"reflect"
	"testing"
)

// 危险命令分类测试（F-06/F-07：十大危险命令拦截 100% 覆盖）
func TestClassifyDangerous(t *testing.T) {
	cases := []struct {
		line string
		want Class
	}{
		{"FLUSHALL", ClassDangerous},
		{"flushall ASYNC", ClassDangerous},
		{"FLUSHDB", ClassDangerous},
		{"SHUTDOWN NOSAVE", ClassDangerous},
		{"DEBUG SLEEP 10", ClassDangerous},     // DEBUG 通用危险
		{"DEBUG OBJECT foo", ClassRead},        // 子命令覆写：只读
		{"CONFIG SET maxmemory 1gb", ClassDangerous},
		{"CONFIG GET maxmemory", ClassRead},
		{"CONFIG", ClassDangerous},
		{"CONFIG RESETSTAT", ClassDangerous},
		{"SLAVEOF NO ONE", ClassDangerous},
		{"REPLICAOF 1.2.3.4 6379", ClassDangerous},
		{"SWAPDB 0 1", ClassDangerous},
		{"MIGRATE host port key 0 100", ClassDangerous},
		{"SCRIPT FLUSH", ClassDangerous},
		{"FUNCTION FLUSH", ClassDangerous},
		{"CLUSTER RESET", ClassDangerous},
		{"CLUSTER INFO", ClassRead},
		{"ACL SETUSER admin on", ClassDangerous},
		{"ACL WHOAMI", ClassRead},
		{"MONITOR", ClassStreaming},
		{"KEYS *", ClassStreaming}, // KEYS 按阻塞命令管控
		{"CLIENT KILL id 123", ClassDangerous},
		{"CLIENT LIST", ClassRead},
		{"SLOWLOG RESET", ClassDangerous},
		{"SLOWLOG GET 10", ClassRead},
	}
	for _, c := range cases {
		args := ParseCommand(c.line)
		got, _ := Classify(args)
		if got != c.want {
			t.Errorf("Classify(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

// 只读模式判断
func TestReadOnlyCommand(t *testing.T) {
	readCases := []string{
		"GET foo", "HGETALL user:1", "LRANGE list 0 -1", "INFO", "DBSIZE",
		"SCAN 0", "TTL foo", "PING", "CONFIG GET maxmemory", "MEMORY USAGE foo",
	}
	for _, line := range readCases {
		if !IsReadOnlyCommand(ParseCommand(line)) {
			t.Errorf("%q should be read-only", line)
		}
	}
	writeCases := []string{
		"SET foo bar", "DEL foo", "EXPIRE foo 100", "FLUSHALL", "HSET foo bar baz",
		"UNKNOWNCMD a b", "EVAL \"return 1\" 0", "PUBLISH ch msg",
	}
	for _, line := range writeCases {
		if IsReadOnlyCommand(ParseCommand(line)) {
			t.Errorf("%q should NOT be read-only", line)
		}
	}
}

// 命令解析：引号、转义、空白
func TestParseCommand(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{`SET foo bar`, []string{"SET", "foo", "bar"}},
		{`SET "key with space" value`, []string{"SET", "key with space", "value"}},
		{`SET 'single quoted' v`, []string{"SET", "single quoted", "v"}},
		{"SET k \"multi\\nline\"", []string{"SET", "k", "multi\nline"}},
		{`GET    multiple   spaces`, []string{"GET", "multiple", "spaces"}},
		{``, nil},
		{`   `, nil},
	}
	for _, c := range cases {
		got := ParseCommand(c.line)
		if len(got) != len(c.want) {
			t.Errorf("ParseCommand(%q) = %v, want %v", c.line, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("ParseCommand(%q)[%d] = %q, want %q", c.line, i, got[i], c.want[i])
			}
		}
	}
}

// 危险命令清单完整性（PRD 十大危险命令）
func TestDangerousCommandsCoverage(t *testing.T) {
	required := []string{
		"FLUSHALL", "FLUSHDB", "SHUTDOWN", "DEBUG", "CONFIG",
		"SLAVEOF", "REPLICAOF", "SWAPDB", "MIGRATE", "KEYS", "MONITOR",
	}
	reg := Registry()
	for _, name := range required {
		cmd, ok := reg[name]
		if !ok {
			t.Errorf("required command %s missing from registry", name)
			continue
		}
		if cmd.Class != ClassDangerous && cmd.Class != ClassStreaming {
			t.Errorf("%s should be dangerous/streaming, got %s", name, cmd.Class)
		}
	}
	if len(DangerousCommands()) < 12 {
		t.Error("dangerous command list too small")
	}
}

func TestClassString(t *testing.T) {
	if reflect.TypeOf(ClassRead).String() == "" {
		t.Error("unexpected")
	}
	if ClassDangerous.String() != "dangerous" {
		t.Errorf("ClassDangerous.String() = %q", ClassDangerous.String())
	}
}
