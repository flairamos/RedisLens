package rds

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"RedisLens/internal/audit"
)

// ExportItem 导出文件中的一条键记录。
type ExportItem struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	PTTL  int64  `json:"pttl"`            // ms；-1 永久
	Dump  string `json:"dump"`            // base64(DUMP)，保真恢复用
	Value string `json:"value,omitempty"` // 人类可读值（字符串/JSON 才填）
}

// ExportHeader 导出文件头。
type ExportHeader struct {
	Format    string    `json:"format"` // RedisLens-export
	Version   int       `json:"version"`
	Source    string    `json:"source"`
	ExportedAt time.Time `json:"exportedAt"`
	Count     int       `json:"count"`
}

// PreviewDelete 批量删除预览：扫描匹配键并汇总（不执行删除）。
func (m *Manager) PreviewDelete(connID string, db int, match, typeFilter string, maxPreview int) (*DeletePreview, error) {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return nil, err
	}
	if maxPreview <= 0 || maxPreview > 10000 {
		maxPreview = 1000
	}
	ctx, cancel := opCtx()
	defer cancel()

	res := &DeletePreview{Match: match, Samples: []KeyItem{}}
	var cursor uint64
	for {
		var keys []string
		var next uint64
		keys, next, err = scanWithType(ctx, cli, cursor, match, typeFilter, 1000)
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			res.Total++
			if len(res.Samples) < maxPreview {
				res.Samples = append(res.Samples, KeyItem{Key: k})
			}
		}
		if next == 0 {
			break
		}
		cursor = next
		// 超大匹配提前中断展示
		if res.Total >= 5_000_000 {
			break
		}
	}
	if len(res.Samples) > 0 {
		m.fillKeyMeta(ctx, cli, res.Samples, b)
	}
	return res, nil
}

// DeletePreview 批量删除预览结果。
type DeletePreview struct {
	Match   string    `json:"match"`
	Total   int64     `json:"total"`
	Samples []KeyItem `json:"samples"`
}

// ExportKeys 导出匹配键到 JSON 文件（含 DUMP 保真数据，可跨实例恢复）。
func (m *Manager) ExportKeys(connID string, db int, match, typeFilter, filePath string) (*ExportReport, error) {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	f, err := os.Create(filePath) //#nosec G304 -- 路径来自系统保存对话框，由用户显式选择
	if err != nil {
		return nil, fmt.Errorf("create export file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	report := &ExportReport{File: filePath, StartedAt: time.Now()}
	header := &ExportHeader{Format: "RedisLens-export", Version: 1, Source: b.Cfg.Name, ExportedAt: time.Now()}
	enc := json.NewEncoder(f) // JSONL：每行一条紧凑 JSON

	// 逐键收集后统一写出（写入 header 占位再回填 count 太复杂；先扫完写文件）
	var items []ExportItem
	var cursor uint64
	for {
		var keys []string
		var next uint64
		keys, next, err = scanWithType(ctx, cli, cursor, match, typeFilter, 1000)
		if err != nil {
			return report, err
		}
		for _, k := range keys {
			typ, err := cli.Type(ctx, k).Result()
			if err != nil || typ == "none" {
				continue
			}
			typ = normalizeType(typ)
			dump, err := cli.Dump(ctx, k).Bytes()
			if err != nil {
				continue
			}
			pttl, _ := cli.PTTL(ctx, k).Result()
			it := ExportItem{
				Key:  k,
				Type: typ,
				PTTL: ttlToMs(pttl),
				Dump: base64.StdEncoding.EncodeToString(dump),
			}
			if typ == "string" {
				if v, err := cli.Get(ctx, k).Result(); err == nil {
					it.Value = v
				}
			} else if typ == "json" {
				if v, err := cli.Do(ctx, "JSON.GET", k, "$").Result(); err == nil {
					it.Value = fmt.Sprintf("%v", v)
				}
			}
			items = append(items, it)
			report.Keys++
		}
		if next == 0 {
			break
		}
		cursor = next
		if ctx.Err() != nil {
			return report, ctx.Err()
		}
	}
	header.Count = len(items)
	if err := enc.Encode(header); err != nil {
		return report, err
	}
	for _, it := range items {
		if err := enc.Encode(it); err != nil {
			return report, err
		}
	}
	report.FinishedAt = time.Now()
	report.DurationMs = report.FinishedAt.Sub(report.StartedAt).Milliseconds()
	m.audit.Log(audit.Entry{
		ConnID: b.ID, ConnName: b.Cfg.Name, Category: "export",
		Action: "EXPORT", Target: filepath.Base(filePath), Detail: match,
		Result: "ok", Level: "info",
	})
	return report, nil
}

// ExportReport 导出报告。
type ExportReport struct {
	File       string    `json:"file"`
	Keys       int       `json:"keys"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	DurationMs int64     `json:"durationMs"`
}

// ImportFile 导入 JSON 导出文件到当前实例。
// 优先 RESTORE（保真，含二进制），失败时回退到按类型重建（仅字符串/JSON）。
func (m *Manager) ImportFile(connID string, db int, filePath string, overwrite bool) (*ImportReport, error) {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return nil, err
	}
	if err := m.assertWritable(b, "IMPORT"); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	raw, err := os.ReadFile(filePath) //#nosec G304 -- 路径来自系统打开文件对话框，由用户显式选择
	if err != nil {
		return nil, err
	}
	report := &ImportReport{File: filePath, StartedAt: time.Now()}
	// 逐行解析 JSONL（导出格式为 header + items 行）
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	var header ExportHeader
	if err := decoder.Decode(&header); err != nil {
		return nil, fmt.Errorf("invalid export file: %w", err)
	}
	if header.Format != "RedisLens-export" {
		return nil, fmt.Errorf("not a RedisLens export file")
	}
	for {
		var it ExportItem
		if err := decoder.Decode(&it); err != nil {
			break // EOF
		}
		report.Total++
		if !overwrite {
			if n, _ := cli.Exists(ctx, it.Key).Result(); n > 0 {
				report.Skipped++
				continue
			}
		}
		dump, err := base64.StdEncoding.DecodeString(it.Dump)
		if err != nil {
			report.Failed++
			continue
		}
		var ttl time.Duration
		if it.PTTL >= 0 {
			ttl = time.Duration(it.PTTL) * time.Millisecond
		} else {
			ttl = 0 // RESTORE 的 ttl 必须 >= 0，0 表示无过期
		}
		err = cli.RestoreReplace(ctx, it.Key, ttl, string(dump)).Err()
		if err != nil {
			// 回退：字符串/JSON 直接重建
			if it.Type == "string" && it.Value != "" {
				err = cli.Set(ctx, it.Key, it.Value, 0).Err()
			} else if it.Type == "json" && it.Value != "" {
				err = cli.Do(ctx, "JSON.SET", it.Key, "$", it.Value).Err()
			}
			if err != nil {
				report.Failed++
				continue
			}
		}
		report.Imported++
	}
	report.FinishedAt = time.Now()
	report.DurationMs = report.FinishedAt.Sub(report.StartedAt).Milliseconds()
	m.audit.Log(audit.Entry{
		ConnID: b.ID, ConnName: b.Cfg.Name, Category: "import",
		Action: "IMPORT", Target: filepath.Base(filePath),
		Detail: fmt.Sprintf("imported=%d skipped=%d failed=%d", report.Imported, report.Skipped, report.Failed),
		Result: "ok", Level: "info",
	})
	return report, nil
}

// ImportReport 导入报告。
type ImportReport struct {
	File       string    `json:"file"`
	Total      int       `json:"total"`
	Imported   int       `json:"imported"`
	Skipped    int       `json:"skipped"`
	Failed     int       `json:"failed"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	DurationMs int64     `json:"durationMs"`
}

// ExportCSV 导出键列表为 CSV（轻量清单：key,type,ttl_ms,size_bytes）。供报表场景。
func (m *Manager) ExportCSV(connID string, db int, match, typeFilter, filePath string) (*ExportReport, error) {
	cli, b, err := m.dbClient(connID, db)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	f, err := os.Create(filePath) //#nosec G304 -- 路径来自系统保存对话框，由用户显式选择
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck
	if _, err := f.WriteString("key,type,ttl_ms,memory_bytes\n"); err != nil {
		return nil, err
	}
	report := &ExportReport{File: filePath, StartedAt: time.Now()}
	var cursor uint64
	for {
		var keys []string
		var next uint64
		keys, next, err = scanWithType(ctx, cli, cursor, match, typeFilter, 1000)
		if err != nil {
			return report, err
		}
		for _, k := range keys {
			typ, _ := cli.Type(ctx, k).Result()
			ttl, _ := cli.PTTL(ctx, k).Result()
			size, _ := cli.MemoryUsage(ctx, k).Result()
			line := fmt.Sprintf("%q,%s,%d,%d\n", csvEscape(k), typ, ttl.Milliseconds(), size)
			if _, err := f.WriteString(line); err != nil {
				return report, err
			}
			report.Keys++
		}
		if next == 0 {
			break
		}
		cursor = next
	}
	report.FinishedAt = time.Now()
	report.DurationMs = report.FinishedAt.Sub(report.StartedAt).Milliseconds()
	m.audit.Log(audit.Entry{
		ConnID: b.ID, ConnName: b.Cfg.Name, Category: "export",
		Action: "EXPORT_CSV", Target: filepath.Base(filePath), Detail: match,
		Result: "ok", Level: "info",
	})
	return report, nil
}

func csvEscape(s string) string {
	return strings.ReplaceAll(s, `"`, `""`)
}
