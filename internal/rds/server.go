package rds

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"RedisLens/internal/config"
)

const (
	readTimeout  = 15 * time.Second
	writeTimeout = 15 * time.Second
)

func splitHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, 6379, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return host, 6379, nil
	}
	return host, port, nil
}

// encodeTarget 地址表示。
func encodeTarget(host string, port int) string {
	if port == 0 {
		port = 6379
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// parseInfo 将 INFO 输出解析为 map（扁平化 section）。
func parseInfo(raw string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.IndexByte(line, ':'); idx > 0 {
			out[line[:idx]] = line[idx+1:]
		}
	}
	return out
}

// TestConnection 仅建立一次性的测试连接并返回服务端摘要，不进入管理器。
func TestConnection(ctx context.Context, cfg *config.Connection) (*ServerSummary, error) {
	cli, err := buildClient(cfg, nil)
	if err != nil {
		return nil, err
	}
	defer cli.Close() //nolint:errcheck
	if err := cli.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return summarize(ctx, cli, cfg.Mode)
}

// ServerSummary 服务端摘要信息。
type ServerSummary struct {
	Version   string `json:"version"`
	Mode      string `json:"mode"` // standalone | sentinel | cluster
	Role      string `json:"role"`
	OS        string `json:"os"`
	UsedMem   uint64 `json:"usedMem"`
	TotalKeys int64  `json:"totalKeys"`
}

func summarize(ctx context.Context, cli redis.UniversalClient, mode string) (*ServerSummary, error) {
	// 兼容 Redis 4.0-5.x：不支持 INFO 多 section 参数（6.0+ 才支持），统一使用全量 INFO
	info, err := cli.Info(ctx).Result()
	if err != nil {
		return nil, err
	}
	m := parseInfo(info)
	s := &ServerSummary{
		Version:  m["redis_version"],
		Mode:     mode,
		Role:     m["role"],
		OS:       m["os"],
		UsedMem:  parseUint(m["used_memory"]),
		TotalKeys: -1,
	}
	if dbsize, err := cli.DBSize(ctx).Result(); err == nil {
		s.TotalKeys = dbsize
	}
	if m["redis_mode"] != "" {
		s.Mode = m["redis_mode"]
	}
	return s, nil
}

// ttlToMs 归一化 go-redis 的 PTTL 结果：-1ns（永久）/-2ns（不存在）保留语义值。
func ttlToMs(d time.Duration) int64 {
	switch d {
	case -1:
		return -1
	case -2:
		return -2
	}
	return d.Milliseconds()
}

func parseUint(v string) uint64 {
	n, _ := strconv.ParseUint(v, 10, 64)
	return n
}

func parseInt(v string) int64 {
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func parseFloat(v string) float64 {
	f, _ := strconv.ParseFloat(v, 64)
	return f
}
