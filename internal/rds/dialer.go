package rds

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"RedisLens/internal/config"
)

// dialTimeout 网络/SSH 握手超时。
const dialTimeout = 8 * time.Second

// sshDialer 通过既有 SSH 会话为每次拨号建立端口转发通道，
// 天然支持 standalone / sentinel / cluster 的任意目标地址。
type sshDialer struct {
	cfg    *config.SSHConfig
	mu     sync.Mutex
	client *ssh.Client
}

func newSSHDialer(cfg *config.SSHConfig) *sshDialer {
	return &sshDialer{cfg: cfg}
}

// Dial 实现 go-redis Dialer 接口。
func (d *sshDialer) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	client, err := d.ensureClient()
	if err != nil {
		return nil, err
	}
	conn, err := client.Dial(network, addr)
	if err != nil {
		// SSH 会话可能已失效：重连一次再试
		d.invalidate()
		client, err = d.ensureClient()
		if err != nil {
			return nil, err
		}
		conn, err = client.Dial(network, addr)
		if err != nil {
			return nil, fmt.Errorf("ssh forward to %s: %w", addr, err)
		}
	}
	return &connCtx{Conn: conn, ctx: ctx}, nil
}

// Close 关闭 SSH 会话。
func (d *sshDialer) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.client != nil {
		_ = d.client.Close()
		d.client = nil
	}
}

func (d *sshDialer) invalidate() {
	d.mu.Lock()
	if d.client != nil {
		_ = d.client.Close()
		d.client = nil
	}
	d.mu.Unlock()
}

// ensureClient 返回存活的 SSH 会话，必要时新建。
func (d *sshDialer) ensureClient() (*ssh.Client, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.client != nil {
		// 轻量探活
		_, _, err := d.client.SendRequest("keepalive@redislens", true, nil)
		if err == nil {
			return d.client, nil
		}
		_ = d.client.Close()
		d.client = nil
	}
	port := d.cfg.Port
	if port == 0 {
		port = 22
	}
	addr := net.JoinHostPort(d.cfg.Host, fmt.Sprintf("%d", port))
	authCfg, err := sshAuthConfig(d.cfg)
	if err != nil {
		return nil, err
	}
	client, err := ssh.Dial("tcp", addr, authCfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	d.client = client
	return client, nil
}

func sshAuthConfig(sc *config.SSHConfig) (*ssh.ClientConfig, error) {
	cfg := &ssh.ClientConfig{
		User:            sc.User,
		Timeout:         dialTimeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //#nosec G106 -- 桌面工具：SSH 主机指纹校验由用户侧管理，未实现 known_hosts 存储
	}
	switch sc.AuthType {
	case "key":
		raw, err := os.ReadFile(sc.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("read ssh key: %w", err)
		}
		var signer ssh.Signer
		if sc.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(raw, []byte(sc.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(raw)
		}
		if err != nil {
			return nil, fmt.Errorf("parse ssh key: %w", err)
		}
		cfg.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	default:
		if sc.Password == "" {
			return nil, fmt.Errorf("ssh password is empty")
		}
		cfg.Auth = []ssh.AuthMethod{ssh.Password(sc.Password)}
	}
	return cfg, nil
}

// connCtx 使 net.Conn 感知拨号上下文取消。
type connCtx struct {
	net.Conn
	ctx context.Context
}

func (c *connCtx) Read(b []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.Conn.Read(b)
}
