// Package config 提供应用配置与连接凭据的本地持久化。
// 凭据使用 AES-256-GCM 加密，主密钥由首次运行生成的随机密钥文件
// 与机器标识（Machine GUID）经 HKDF 派生，密文与密钥文件分离存储。
package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/hkdf"
)

const (
	encPrefix    = "AESGCM1:" // 密文前缀，便于将来算法升级
	keyFileSize  = 32
	aesNonceSize = 12
)

// ErrNotEncrypted 表示字符串不是本工具加密的密文。
var ErrNotEncrypted = errors.New("value is not encrypted")

// Crypter 提供 AES-256-GCM 的字符串加密解密能力。
type Crypter struct {
	aead cipher.AEAD
}

// NewCrypter 基于 dir 下的 key.bin 与机器标识派生密钥。
func NewCrypter(dir string) (*Crypter, error) {
	key, err := loadOrCreateMasterKey(filepath.Join(dir, "key.bin"))
	if err != nil {
		return nil, fmt.Errorf("load master key: %w", err)
	}
	machineID := machineIdentity()
	// HKDF-SHA256: master key + machine identity -> AES-256 key
	hk := hkdf.New(sha256.New, key, []byte("RedisLens/v1/credentials"), []byte(machineID))
	aesKey := make([]byte, 32)
	if _, err := io.ReadFull(hk, aesKey); err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return &Crypter{aead: aead}, nil
}

// Encrypt 将明文加密为可存入 JSON 的字符串；空串原样返回。
func (c *Crypter) Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	nonce := make([]byte, aesNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}
	sealed := c.aead.Seal(nil, nonce, []byte(plain), nil)
	out := make([]byte, 0, len(nonce)+len(sealed))
	out = append(out, nonce...)
	out = append(out, sealed...)
	return encPrefix + hex.EncodeToString(out), nil
}

// Decrypt 解密 Encrypt 产生的字符串；空串原样返回。
func (c *Crypter) Decrypt(secret string) (string, error) {
	if secret == "" {
		return "", nil
	}
	if !strings.HasPrefix(secret, encPrefix) {
		return "", ErrNotEncrypted
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(secret, encPrefix))
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	if len(raw) < aesNonceSize {
		return "", errors.New("ciphertext too short")
	}
	plain, err := c.aead.Open(nil, raw[:aesNonceSize], raw[aesNonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("gcm open: %w", err)
	}
	return string(plain), nil
}

// loadOrCreateMasterKey 读取或生成 32 字节主密钥文件。
func loadOrCreateMasterKey(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil && len(data) == keyFileSize { //#nosec G304 -- path 由应用自身配置目录拼出，非外部输入
		return data, nil
	}
	key := make([]byte, keyFileSize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil { //#nosec G306 -- 密钥文件仅属主可读
		return nil, fmt.Errorf("write key file: %w", err)
	}
	restrictFileOwner(path)
	return key, nil
}

// machineIdentity 返回稳定的机器标识，防止密钥文件被拷贝到其他机器后解密。
func machineIdentity() string {
	if runtime.GOOS == "windows" {
		if id := windowsMachineGUID(); id != "" {
			return id
		}
	}
	if hn, err := os.Hostname(); err == nil && hn != "" {
		return hn
	}
	return "unknown-host"
}
