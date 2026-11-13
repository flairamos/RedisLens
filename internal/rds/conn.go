package rds

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"

	"github.com/redis/go-redis/v9"

	"RedisLens/internal/config"
)

// dialFunc 与 go-redis Dialer 字段一致的拨号函数类型。
type dialFunc = func(ctx context.Context, network, addr string) (net.Conn, error)

// buildTLS 按 TLS 配置构造 *tls.Config。
func buildTLS(tc config.TLSConfig, serverAddr string) (*tls.Config, error) {
	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: tc.InsecureSkipVeri, //#nosec G402 -- 用户在连接配置中显式勾选"跳过证书校验"（自签证书场景），非默认行为
	}
	if tc.ServerName != "" {
		cfg.ServerName = tc.ServerName
	}
	if tc.CertFile != "" && tc.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tc.CertFile, tc.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	if tc.CAFile != "" {
		raw, err := os.ReadFile(tc.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(raw) {
			return nil, fmt.Errorf("no valid certificate in CA file %s", tc.CAFile)
		}
		cfg.RootCAs = pool
	}
	if cfg.ServerName == "" && !cfg.InsecureSkipVerify {
		host, _, _ := splitHostPort(serverAddr)
		cfg.ServerName = host
	}
	return cfg, nil
}

// buildClient 依据连接配置构造 go-redis 客户端。
func buildClient(cfg *config.Connection, dialer dialFunc) (redis.UniversalClient, error) {
	var tlsCfg *tls.Config
	if cfg.TLS.Enabled {
		var err error
		tlsCfg, err = buildTLS(cfg.TLS, encodeTarget(cfg.Host, cfg.Port))
		if err != nil {
			return nil, err
		}
	}
	base := &redis.Options{
		Addr:                  encodeTarget(cfg.Host, cfg.Port),
		Username:              cfg.Username,
		Password:              cfg.Password,
		DB:                    cfg.DB,
		TLSConfig:             tlsCfg,
		Dialer:                dialer,
		DialTimeout:           dialTimeout,
		ReadTimeout:           readTimeout,
		WriteTimeout:          writeTimeout,
		PoolSize:              16,
		MaxRetries:            2,
		Protocol:              0, // 服务端协商
		DisableIndentity:      false,
		ContextTimeoutEnabled: true,
	}
	switch cfg.Mode {
	case "sentinel":
		opts := &redis.FailoverOptions{
			MasterName:            cfg.Sentinel.Master,
			SentinelAddrs:         cfg.Sentinel.Addrs,
			SentinelUsername:      cfg.Sentinel.Username,
			SentinelPassword:      cfg.Sentinel.Password,
			Username:              cfg.Username,
			Password:              cfg.Password,
			DB:                    cfg.DB,
			TLSConfig:             tlsCfg,
			Dialer:                dialer,
			DialTimeout:           dialTimeout,
			ReadTimeout:           readTimeout,
			WriteTimeout:          writeTimeout,
			PoolSize:              16,
			MaxRetries:            2,
			ContextTimeoutEnabled: true,
		}
		return redis.NewFailoverClient(opts), nil
	case "cluster":
		opts := &redis.ClusterOptions{
			Addrs:                 []string{encodeTarget(cfg.Host, cfg.Port)},
			Username:              cfg.Username,
			Password:              cfg.Password,
			TLSConfig:             tlsCfg,
			Dialer:                dialer,
			DialTimeout:           dialTimeout,
			ReadTimeout:           readTimeout,
			WriteTimeout:          writeTimeout,
			PoolSize:              8,
			MaxRedirects:          3,
			MaxRetries:            2,
			ContextTimeoutEnabled: true,
		}
		return redis.NewClusterClient(opts), nil
	default:
		return redis.NewClient(base), nil
	}
}
