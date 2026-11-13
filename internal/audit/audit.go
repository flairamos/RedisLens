// Package audit 提供本地操作审计日志：JSONL 月度轮转，默认保留 90 天。
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RetentionDays 审计日志留存天数（PRD 非功能需求：≥ 90 天）。
const RetentionDays = 90

// Entry 一条审计记录。
type Entry struct {
	Time     time.Time `json:"time"`
	ConnID   string    `json:"connId"`
	ConnName string    `json:"connName"`
	Category string    `json:"category"` // command | key_delete | key_edit | connection | dangerous
	Action   string    `json:"action"`   // 具体动作，如 DEL、FLUSHALL
	Target   string    `json:"target"`   // 作用对象（键名 / 命令摘要）
	Detail   string    `json:"detail,omitempty"`
	Result   string    `json:"result"` // ok | error | blocked
	Level    string    `json:"level"`  // info | warn | danger
}

// Logger 线程安全的审计日志写入器。
type Logger struct {
	mu      sync.Mutex
	dir     string
	enabled bool
	day     string // 当前写入的日志归属日（YYYY-MM），用于月度轮转
	f       *os.File
}

// New 创建审计日志器；enabled 为 false 时所有写入为空操作。
// 目录不存在时自动创建，避免日志静默丢失。
func New(dir string, enabled bool) *Logger {
	if err := os.MkdirAll(dir, 0o700); err != nil { //nolint:gosec // 审计目录
		enabled = false
	}
	return &Logger{dir: dir, enabled: enabled}
}

// SetEnabled 动态开关审计日志。
func (l *Logger) SetEnabled(v bool) {
	l.mu.Lock()
	l.enabled = v
	l.mu.Unlock()
}

// Enabled 报告当前开关状态。
func (l *Logger) Enabled() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enabled
}

// Log 写入一条审计记录。
func (l *Logger) Log(e Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.enabled {
		return
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	month := e.Time.Format("2006-01")
	if l.f == nil || month != l.day {
		if l.f != nil {
			_ = l.f.Close()
			l.f = nil
		}
		f, err := os.OpenFile(l.pathFor(month), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //#nosec G304 -- path 由应用固定目录与月份常量拼出
		if err != nil {
			return
		}
		l.f, l.day = f, month
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = l.f.Write(append(raw, '\n'))
}

// Close 关闭当前日志文件。
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f != nil {
		_ = l.f.Close()
		l.f = nil
	}
}

// Query 读取最近的审计记录（跨全部日志文件，倒序返回 limit 条）。
func (l *Logger) Query(limit int) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if limit <= 0 {
		limit = 200
	}
	files, err := filepath.Glob(filepath.Join(l.dir, "audit-*.jsonl"))
	if err != nil {
		return nil, err
	}
	// 文件名按月，倒序读取
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
		files[i], files[j] = files[j], files[i]
	}
	var out []Entry
	for _, f := range files {
		raw, err := os.ReadFile(f) //#nosec G304 -- f 来自应用自身审计目录的 Glob，无外部输入
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
		// 倒序收集
		for i := len(lines) - 1; i >= 0 && len(out) < limit; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			var e Entry
			if json.Unmarshal([]byte(line), &e) == nil {
				out = append(out, e)
			}
		}
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// Cleanup 删除超过留存期的旧日志文件。
func (l *Logger) Cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().AddDate(0, 0, -RetentionDays).Format("2006-01")
	files, _ := filepath.Glob(filepath.Join(l.dir, "audit-*.jsonl"))
	for _, f := range files {
		base := strings.TrimPrefix(filepath.Base(f), "audit-")
		base = strings.TrimSuffix(base, ".jsonl")
		if base < cutoff {
			_ = os.Remove(f)
		}
	}
}

func (l *Logger) pathFor(month string) string {
	return filepath.Join(l.dir, fmt.Sprintf("audit-%s.jsonl", month))
}
