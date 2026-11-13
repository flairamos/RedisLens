package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// AppName 应用名，同时用于配置目录名。
const AppName = "RedisLens"

// SSHConfig SSH 隧道配置。
type SSHConfig struct {
	Enabled    bool   `json:"enabled"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	AuthType   string `json:"authType"` // password | key
	Password   string `json:"password,omitempty"`
	KeyPath    string `json:"keyPath,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

// TLSConfig TLS 加密配置。
type TLSConfig struct {
	Enabled          bool   `json:"enabled"`
	CertFile         string `json:"certFile,omitempty"`
	KeyFile          string `json:"keyFile,omitempty"`
	CAFile           string `json:"caFile,omitempty"`
	ServerName       string `json:"serverName,omitempty"`
	InsecureSkipVeri bool   `json:"insecureSkipVerify,omitempty"`
}

// SentinelConfig 哨兵模式配置。
type SentinelConfig struct {
	Master   string   `json:"master"`
	Addrs    []string `json:"addrs"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
}

// Connection 一条 Redis 实例连接。机密字段仅在内存中为明文，落盘前由 Store 统一加密。
type Connection struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Group        string         `json:"group"` // 环境分组，如 开发/测试/生产
	Mode         string         `json:"mode"`  // standalone | sentinel | cluster
	Host         string         `json:"host"`
	Port         int            `json:"port"`
	Username     string         `json:"username,omitempty"`
	Password     string         `json:"-"` // 内存态明文
	DB           int            `json:"db"`
	ReadOnly     bool           `json:"readOnly"`
	SSHTunnel    SSHConfig      `json:"ssh"`
	TLS          TLSConfig      `json:"tls"`
	Sentinel     SentinelConfig `json:"sentinel,omitempty"`
	SentinelPass string         `json:"-"`
	CreatedAt    time.Time      `json:"createdAt"`
	LastUsedAt   time.Time      `json:"lastUsedAt"`
}

// Settings 应用全局设置。
type Settings struct {
	Theme              string `json:"theme"`              // light | dark
	Density            string `json:"density"`            // compact | comfortable | spacious
	MonitorIntervalSec int    `json:"monitorIntervalSec"` // 采样间隔，默认 10
	ConfirmDanger      bool   `json:"confirmDanger"`      // 危险命令二次确认，默认 true
	LogEnabled         bool   `json:"logEnabled"`         // 本地日志开关
	ScanCount          int    `json:"scanCount"`          // SCAN COUNT，默认 500
	ValuePreviewLimit  int    `json:"valuePreviewLimit"`  // 大值预览字节上限
}

// Defaults 返回默认设置。
func Defaults() Settings {
	return Settings{
		Theme:              "light",
		Density:            "comfortable",
		MonitorIntervalSec: 10,
		ConfirmDanger:      true,
		LogEnabled:         true,
		ScanCount:          500,
		ValuePreviewLimit:  2 << 20, // 2 MiB
	}
}

// storeData 内存态数据：机密字段为明文（进程内），仅保存时加密。
type storeData struct {
	Version     int           `json:"version"`
	Connections []*Connection `json:"connections"`
	Settings    Settings      `json:"settings"`
}

// connectionFS 磁盘上的连接结构：机密字段为 AES-256-GCM 密文。
type connectionFS struct {
	*Connection
	PasswordEnc     string `json:"passwordEnc,omitempty"`
	SentinelPassEnc string `json:"sentinelPassEnc,omitempty"`
	SSHPasswordEnc  string `json:"sshPasswordEnc,omitempty"`
	SSHKeyPassEnc   string `json:"sshKeyPassEnc,omitempty"`
}

// diskData 落盘结构。
type diskData struct {
	Version     int             `json:"version"`
	Connections []*connectionFS `json:"connections"`
	Settings    Settings        `json:"settings"`
}

// Store 配置存储，线程安全。
type Store struct {
	mu      sync.RWMutex
	dir     string
	file    string
	crypter *Crypter
	data    *storeData
}

// Open 打开（不存在则初始化）配置目录并加载配置。
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil { //nolint:gosec // 用户配置目录
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	crypter, err := NewCrypter(dir)
	if err != nil {
		return nil, err
	}
	s := &Store{
		dir:     dir,
		file:    filepath.Join(dir, "config.json"),
		crypter: crypter,
		data: &storeData{
			Version:     1,
			Settings:    Defaults(),
			Connections: []*Connection{},
		},
	}
	if raw, err := os.ReadFile(s.file); err == nil {
		s.loadFrom(raw)
	}
	return s, nil
}

// loadFrom 解析磁盘数据并解密机密字段；损坏时备份原文件。
func (s *Store) loadFrom(raw []byte) {
	var disk diskData
	if err := json.Unmarshal(raw, &disk); err != nil {
		// 备份损坏文件，避免覆盖丢失
		_ = os.Rename(s.file, s.file+".corrupt."+time.Now().Format("20060102150405"))
		return
	}
	s.data.Version = disk.Version
	if disk.Settings.Theme != "" {
		s.data.Settings = disk.Settings
	}
	for _, fs := range disk.Connections {
		c := fs.Connection
		if c == nil {
			continue
		}
		c.Password, _ = s.crypter.Decrypt(fs.PasswordEnc)
		c.SentinelPass, _ = s.crypter.Decrypt(fs.SentinelPassEnc)
		c.SSHTunnel.Password, _ = s.crypter.Decrypt(fs.SSHPasswordEnc)
		c.SSHTunnel.Passphrase, _ = s.crypter.Decrypt(fs.SSHKeyPassEnc)
		s.data.Connections = append(s.data.Connections, c)
	}
}

// Dir 返回配置目录。
func (s *Store) Dir() string { return s.dir }

// Decrypt 用于解密落盘密文（供迁移等场景）。
func (s *Store) Decrypt(v string) (string, error) { return s.crypter.Decrypt(v) }

// Save 将配置加密落盘。
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	enc := func(v string) string {
		if v == "" {
			return ""
		}
		out, err := s.crypter.Encrypt(v)
		if err != nil {
			return ""
		}
		return out
	}
	out := &diskData{Version: s.data.Version, Settings: s.data.Settings, Connections: make([]*connectionFS, 0, len(s.data.Connections))}
	for _, c := range s.data.Connections {
		// 深拷贝并置空机密字段：embedded 结构的密码字段带 json tag，必须清空防止明文落盘
		cp := *c
		cp.Password = ""
		cp.SentinelPass = ""
		cp.SSHTunnel.Password = ""
		cp.SSHTunnel.Passphrase = ""
		fs := &connectionFS{Connection: &cp}
		fs.PasswordEnc = enc(c.Password)
		fs.SentinelPassEnc = enc(c.SentinelPass)
		fs.SSHPasswordEnc = enc(c.SSHTunnel.Password)
		fs.SSHKeyPassEnc = enc(c.SSHTunnel.Passphrase)
		out.Connections = append(out.Connections, fs)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := s.file + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil { //#nosec G306 -- 配置含机密，仅属主可读
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, s.file); err != nil {
		return fmt.Errorf("rename config: %w", err)
	}
	return nil
}

// ---------- 连接 CRUD ----------

// ListConnections 返回按分组、名称排序的连接列表（含内存态明文机密，勿外泄）。
func (s *Store) ListConnections() []*Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Connection, 0, len(s.data.Connections))
	out = append(out, s.data.Connections...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// GetConnection 按 ID 查找连接（拷贝）。
func (s *Store) GetConnection(id string) (*Connection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.data.Connections {
		if c.ID == id {
			cp := *c
			return &cp, true
		}
	}
	return nil, false
}

// UpsertConnection 新增或更新连接并落盘。
func (s *Store) UpsertConnection(c *Connection) error {
	if c.ID == "" {
		return errors.New("connection id is empty")
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	c.LastUsedAt = time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	replaced := false
	for i, old := range s.data.Connections {
		if old.ID == c.ID {
			s.data.Connections[i] = c
			replaced = true
			break
		}
	}
	if !replaced {
		s.data.Connections = append(s.data.Connections, c)
	}
	return s.saveLocked()
}

// DeleteConnection 删除连接并落盘。
func (s *Store) DeleteConnection(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.data.Connections[:0]
	for _, c := range s.data.Connections {
		if c.ID != id {
			kept = append(kept, c)
		}
	}
	s.data.Connections = kept
	return s.saveLocked()
}

// TouchConnection 更新最近使用时间。
func (s *Store) TouchConnection(id string) {
	s.mu.Lock()
	for _, c := range s.data.Connections {
		if c.ID == id {
			c.LastUsedAt = time.Now()
		}
	}
	s.mu.Unlock()
	_ = s.saveLocked()
}

// Settings 返回设置拷贝。
func (s *Store) Settings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Settings
}

// UpdateSettings 更新设置并落盘。
func (s *Store) UpdateSettings(st Settings) error {
	if st.MonitorIntervalSec < 1 {
		st.MonitorIntervalSec = 1
	}
	if st.ScanCount < 10 {
		st.ScanCount = 10
	}
	if st.ValuePreviewLimit < 4096 {
		st.ValuePreviewLimit = 4096
	}
	s.mu.Lock()
	s.data.Settings = st
	defer s.mu.Unlock()
	return s.saveLocked()
}
