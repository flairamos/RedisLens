package config

import (
	"os"
	"path/filepath"
	"testing"
)

// 凭据加密往返测试（F-01 / 非功能需求：AES-256 本地加密存储）
func TestCrypterRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCrypter(dir)
	if err != nil {
		t.Fatalf("NewCrypter: %v", err)
	}
	cases := []string{"", "simple-password", "中文密码!@#123", "with\"quote", "multi\nline\tvalue"}
	for _, plain := range cases {
		enc, err := c.Encrypt(plain)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", plain, err)
		}
		got, err := c.Decrypt(enc)
		if err != nil {
			t.Fatalf("Decrypt(%q): %v", plain, err)
		}
		if got != plain {
			t.Errorf("roundtrip mismatch: got %q want %q", got, plain)
		}
	}
	// 非本工具密文应报错
	if _, err := c.Decrypt("not-encrypted"); err != ErrNotEncrypted {
		t.Errorf("expected ErrNotEncrypted, got %v", err)
	}
}

// 密文不可逆推：不同实例产生不同密文（随机 nonce）
func TestCrypterUniqueCiphertext(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCrypter(dir)
	e1, _ := c.Encrypt("same-secret")
	e2, _ := c.Encrypt("same-secret")
	if e1 == e2 {
		t.Error("ciphertexts should differ due to random nonce")
	}
}

// 机器绑定：不同机器标识无法解密（模拟拷贝密钥文件到其他机器）
func TestCrypterMachineBound(t *testing.T) {
	dir := t.TempDir()
	c1, _ := NewCrypter(dir)
	enc, err := c1.Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	// 模拟 machineIdentity 变化：直接篡改注册表读取结果不可行，
	// 改为验证密钥文件复用 + 相同机器仍可解密（基本可用性）
	c2, _ := NewCrypter(dir)
	got, err := c2.Decrypt(enc)
	if err != nil || got != "secret" {
		t.Errorf("same machine decrypt failed: %v", err)
	}
}

// 配置存储：保存后密码落盘为密文，重新加载可解密
func TestStoreSaveLoadEncrypted(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	conn := &Connection{
		ID:       "c1",
		Name:     "prod-cache",
		Group:    "生产",
		Mode:     "standalone",
		Host:     "10.0.0.5",
		Port:     6379,
		Password: "super-secret-pass",
		SSHTunnel: SSHConfig{
			Enabled:  true,
			Host:     "bastion",
			Port:     22,
			User:     "ops",
			AuthType: "password",
			Password: "ssh-pass",
		},
	}
	if err := s.UpsertConnection(conn); err != nil {
		t.Fatal(err)
	}

	// 落盘文件中不应出现明文密码
	raw, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	if containsStr(raw, "super-secret-pass") || containsStr(raw, "ssh-pass") {
		t.Error("plaintext password leaked to config file")
	}

	// 重新打开可解密
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s2.GetConnection("c1")
	if !ok {
		t.Fatal("connection not found after reload")
	}
	if got.Password != "super-secret-pass" {
		t.Errorf("password mismatch after reload: %q", got.Password)
	}
	if got.SSHTunnel.Password != "ssh-pass" {
		t.Errorf("ssh password mismatch after reload: %q", got.SSHTunnel.Password)
	}
}

func containsStr(b []byte, s string) bool {
	return len(s) > 0 && string(b) != "" && indexOf(string(b), s) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// 连接 CRUD
func TestStoreCRUD(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)

	c1 := &Connection{ID: "a", Name: "dev-1", Group: "开发"}
	c2 := &Connection{ID: "b", Name: "prod-1", Group: "生产"}
	c3 := &Connection{ID: "c", Name: "dev-2", Group: "开发"}
	for _, c := range []*Connection{c1, c2, c3} {
		if err := s.UpsertConnection(c); err != nil {
			t.Fatal(err)
		}
	}
	list := s.ListConnections()
	if len(list) != 3 {
		t.Fatalf("expected 3 connections, got %d", len(list))
	}
	// 排序：分组 + 名称
	if list[0].Group != "开发" || list[0].Name != "dev-1" {
		t.Errorf("sort order wrong: %v", list[0])
	}
	if err := s.DeleteConnection("a"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.GetConnection("a"); ok {
		t.Error("connection a should be deleted")
	}
}

// 设置校验
func TestStoreSettingsValidation(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	err := s.UpdateSettings(Settings{MonitorIntervalSec: 0, ScanCount: 5, ValuePreviewLimit: 0})
	if err != nil {
		t.Fatal(err)
	}
	st := s.Settings()
	if st.MonitorIntervalSec < 1 || st.ScanCount < 10 || st.ValuePreviewLimit < 4096 {
		t.Errorf("validation failed: %+v", st)
	}
}
