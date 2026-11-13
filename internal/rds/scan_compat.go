package rds

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"
)

// scanWithType 带类型过滤的 SCAN，兼容 Redis 4.0-5.x：
// Redis 6.0+ 使用 SCAN TYPE；低版本报语法错误时降级为普通 SCAN +
// 客户端 TYPE 过滤（扫描量略增，仍为渐进遍历，无阻塞）。
func scanWithType(ctx context.Context, cli redis.UniversalClient, cursor uint64, match, typeFilter string, count int64) ([]string, uint64, error) {
	if typeFilter == "" {
		return cli.Scan(ctx, cursor, match, count).Result()
	}
	keys, next, err := cli.ScanType(ctx, cursor, match, count, typeFilter).Result()
	if err == nil {
		return keys, next, nil
	}
	if !isSyntaxErr(err) {
		return nil, 0, err
	}
	// 降级：SCAN 后逐键 TYPE 过滤。过滤后可能为空页，内部继续翻页
	// 凑批（上限 20 页 / 2 万键，防止失控），保证调用方尽量拿到非空结果。
	var out []string
	page := 0
	for {
		keys, next2, err := cli.Scan(ctx, cursor, match, count).Result()
		if err != nil {
			return nil, 0, err
		}
		cursor = next2
		if len(keys) > 0 {
			pipe := cli.Pipeline()
			cmds := make([]*redis.StatusCmd, len(keys))
			for i, k := range keys {
				cmds[i] = pipe.Type(ctx, k)
			}
			if _, perr := pipe.Exec(ctx); perr != nil {
				return nil, 0, perr
			}
			for i, k := range keys {
				if t, e := cmds[i].Result(); e == nil && strings.EqualFold(t, typeFilter) {
					out = append(out, k)
				}
			}
		}
		page++
		// 游标归零 / 已凑够一页结果 / 达到翻页上限 → 结束
		if cursor == 0 || len(out) >= int(count) || page >= 20 {
			break
		}
	}
	return out, cursor, nil
}

func isSyntaxErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "syntax error") ||
		strings.Contains(msg, "unknown command 'scan'") ||
		strings.Contains(msg, "unknown command `scan`")
}
