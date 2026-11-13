package rds

import (
	"context"
	"fmt"
	"strings"
	"time"

	"RedisLens/internal/audit"
	"RedisLens/internal/security"
)

// ConsoleResult 控制台命令执行结果。
type ConsoleResult struct {
	OK       bool     `json:"ok"`
	Blocked  bool     `json:"blocked"`         // 危险命令被拦截
	NeedConf bool     `json:"needConfirm"`     // 需要二次确认
	Reason   string   `json:"reason,omitempty"`
	Op       string   `json:"op,omitempty"`    // 命令名（拦截提示用）
	Reply    string   `json:"reply"`           // 格式化后的回复
	Rows     []string `json:"rows,omitempty"`  // 结构化行（表格展示）
	Duration int64    `json:"duration"`        // 执行耗时 ms
}

// guardReason 拦截原因。
type guardReason int

const (
	guardOK guardReason = iota
	guardDangerous      // 危险命令：需确认
	guardReadOnly       // 只读连接拦截写命令
	guardStreaming      // 流式命令拦截
)

// checkCommand 执行前置检查。
func (m *Manager) checkCommand(b *Bundle, args []string) (guardReason, string) {
	class, cmd := security.Classify(args)
	op := strings.ToUpper(args[0])
	switch class {
	case security.ClassDangerous:
		return guardDangerous, fmt.Sprintf("「%s」是危险命令，可能造成不可逆的数据或服务影响", op)
	case security.ClassStreaming:
		return guardStreaming, fmt.Sprintf("「%s」为流式/阻塞命令，会独占连接，请谨慎使用", op)
	case security.ClassWrite:
		if b.ReadOnly {
			return guardReadOnly, "当前连接处于只读模式，已拦截写命令"
		}
	case security.ClassUnknown:
		if b.ReadOnly {
			// 未知命令保守按写拦截
			return guardReadOnly, "当前连接处于只读模式，未知命令已拦截"
		}
	}
	_ = cmd
	return guardOK, ""
}

// ExecCommand 执行控制台命令。
// confirm=true 表示用户已通过二次确认，危险命令放行并强制审计。
func (m *Manager) ExecCommand(connID, line string, confirm bool) (*ConsoleResult, error) {
	b, err := m.Get(connID)
	if err != nil {
		return nil, err
	}
	cli := b.Client
	if cli == nil {
		return nil, fmt.Errorf("connection is down")
	}
	args := security.ParseCommand(line)
	if len(args) == 0 {
		return &ConsoleResult{OK: true, Reply: ""}, nil
	}

	reason, msg := m.checkCommand(b, args)
	if reason != guardOK && !(confirm && reason != guardReadOnly) {
		res := &ConsoleResult{Blocked: true, Reason: msg, Op: strings.ToUpper(args[0])}
		if reason == guardDangerous || reason == guardStreaming {
			res.NeedConf = true
		}
		m.audit.Log(audit.Entry{
			ConnID: b.ID, ConnName: b.Cfg.Name, Category: "dangerous",
			Action: res.Op, Target: line, Result: "blocked", Detail: msg, Level: "warn",
		})
		return res, nil
	}
	if reason == guardDangerous || reason == guardStreaming {
		// 放行的危险命令强制审计
		m.audit.Log(audit.Entry{
			ConnID: b.ID, ConnName: b.Cfg.Name, Category: "dangerous",
			Action: strings.ToUpper(args[0]), Target: line,
			Result: "ok", Detail: "user confirmed", Level: "danger",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	cmdArgs := make([]any, len(args))
	for i, a := range args {
		cmdArgs[i] = a
	}
	raw, err := cli.Do(ctx, cmdArgs...).Result()
	dur := time.Since(start).Milliseconds()

	res := &ConsoleResult{Duration: dur}
	if err != nil {
		if isNilReply(err) {
			res.OK = true
			res.Reply = "(nil)"
		} else {
			res.OK = false
			res.Reply = formatRedisError(err)
		}
	} else {
		res.OK = true
		formatReply(raw, res)
	}
	m.audit.Log(audit.Entry{
		ConnID: b.ID, ConnName: b.Cfg.Name, Category: "command",
		Action: strings.ToUpper(args[0]), Target: line,
		Result: map[bool]string{true: "ok", false: "error"}[res.OK], Level: "info",
	})
	return res, nil
}

func isNilReply(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "nil")
}

func formatRedisError(err error) string {
	msg := err.Error()
	msg = strings.TrimPrefix(msg, "ERR ")
	return "ERR " + msg
}

// formatReply 将任意 RESP 回复格式化为展示文本与结构化行。
func formatReply(v any, res *ConsoleResult) {
	switch t := v.(type) {
	case nil:
		res.Reply = "(nil)"
	case string:
		res.Reply = quoteIfNeeded(t)
	case int64:
		res.Reply = fmt.Sprintf("(integer) %d", t)
	case int:
		res.Reply = fmt.Sprintf("(integer) %d", t)
	case float64:
		res.Reply = fmt.Sprintf("(float) %v", t)
	case []any:
		if len(t) == 0 {
			res.Reply = "(empty array)"
			return
		}
		var sb strings.Builder
		for i, item := range t {
			sb.WriteString(fmt.Sprintf("%d) %s\n", i+1, formatOne(item)))
		}
		res.Reply = strings.TrimRight(sb.String(), "\n")
	default:
		res.Reply = fmt.Sprintf("%v", t)
	}
}

func formatOne(v any) string {
	switch t := v.(type) {
	case nil:
		return "(nil)"
	case string:
		return quoteIfNeeded(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, formatOne(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("%v", t)
	}
}

func quoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, "\n\r\t ") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// CompleteCommand 键名补全：SCAN 匹配前缀。
func (m *Manager) CompleteCommand(connID string, db int, prefix string) ([]string, error) {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	keys, _, err := cli.Scan(ctx, 0, prefix+"*", 50).Result()
	if err != nil {
		return nil, err
	}
	_ = b
	return keys, nil
}

// ExportCommandHelp 导出命令注册表给前端补全。
func ExportCommandHelp() []CommandHelp {
	var out []CommandHelp
	for _, cmd := range security.Registry() {
		out = append(out, CommandHelp{
			Name:   cmd.Name,
			Group:  cmd.Group,
			Synop:  cmd.Synop,
			Danger: cmd.Class.String() == "dangerous" || cmd.Class.String() == "streaming",
		})
	}
	return out
}

// CommandHelp 命令补全元数据。
type CommandHelp struct {
	Name   string `json:"name"`
	Group  string `json:"group"`
	Synop  string `json:"synop"`
	Danger bool   `json:"danger"`
}
