package rds

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// PubSubMessage 订阅消息。
type PubSubMessage struct {
	ConnID    string    `json:"connId"`
	Kind      string    `json:"kind"` // message | pmessage | smessage | subscribe | unsubscribe
	Channel   string    `json:"channel"`
	Pattern   string    `json:"pattern,omitempty"`
	Payload   string    `json:"payload"`
	Binary    bool      `json:"binary"`
	Timestamp int64     `json:"ts"`
}

// PubSubManager 每连接一个订阅管理器。
type PubSubManager struct {
	mu      sync.Mutex
	connID  string
	client  redis.UniversalClient
	pubsub  *redis.PubSub
	emit    func(event string, payload any)
	active  bool
	cancel  context.CancelFunc
}

// newPubSubManager 创建订阅管理器（不立即连接）。
func newPubSubManager(connID string, client redis.UniversalClient, emit func(string, any)) *PubSubManager {
	return &PubSubManager{connID: connID, client: client, emit: emit}
}

// Subscribe 订阅频道（普通 + 模式订阅）。
func (p *PubSubManager) Subscribe(ctx context.Context, channels []string, patterns []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 已有订阅则重建
	if p.pubsub != nil {
		_ = p.pubsub.Close()
		p.pubsub = nil
	}
	ps := p.client.Subscribe(ctx, channels...)
	if len(patterns) > 0 {
		_ = ps.PSubscribe(ctx, patterns...)
	}
	p.pubsub = ps
	ctx2, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.active = true
	go p.readLoop(ctx2)
	return nil
}

// Unsubscribe 停止订阅。
func (p *PubSubManager) Unsubscribe() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.active = false
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	if p.pubsub != nil {
		_ = p.pubsub.Close()
		p.pubsub = nil
	}
}

func (p *PubSubManager) readLoop(ctx context.Context) {
	for {
		msg, err := p.pubsub.Receive(ctx)
		if err != nil {
			return // 上下文取消或连接关闭
		}
		switch m := msg.(type) {
		case *redis.Message:
			payload, binary := bytesToVal([]byte(m.Payload))
			kind := "message"
			if m.Pattern != "" {
				kind = "pmessage"
			}
			p.emit("pubsub:message", PubSubMessage{
				ConnID: p.connID, Kind: kind, Channel: m.Channel, Pattern: m.Pattern,
				Payload: payload, Binary: binary, Timestamp: time.Now().UnixMilli(),
			})
		case *redis.Subscription:
			p.emit("pubsub:message", PubSubMessage{
				ConnID: p.connID, Kind: m.Kind, Channel: m.Channel,
				Payload: "", Timestamp: time.Now().UnixMilli(),
			})
		}
	}
}

// Publish 发布消息。
func (m *Manager) Publish(connID, channel, payload string) (int64, error) {
	b, err := m.Get(connID)
	if err != nil {
		return 0, err
	}
	if err := m.assertWritable(b, "PUBLISH"); err != nil {
		return 0, err
	}
	cli := b.Client
	if cli == nil {
		return 0, fmt.Errorf("connection is down")
	}
	ctx, cancel := opCtx()
	defer cancel()
	return cli.Publish(ctx, channel, payload).Result()
}

// PubSubSubscribe 开启订阅会话。
func (m *Manager) PubSubSubscribe(connID string, channels, patterns []string) error {
	b, err := m.Get(connID)
	if err != nil {
		return err
	}
	if b.Client == nil {
		return fmt.Errorf("connection is down")
	}
	ctx, cancel := opCtx()
	defer cancel()
	if b.pubsub == nil {
		b.pubsub = newPubSubManager(connID, b.Client, m.emit)
	}
	return b.pubsub.Subscribe(ctx, channels, patterns)
}

// PubSubUnsubscribe 停止订阅会话。
func (m *Manager) PubSubUnsubscribe(connID string) error {
	b, err := m.Get(connID)
	if err != nil {
		return err
	}
	if b.pubsub != nil {
		b.pubsub.Unsubscribe()
	}
	return nil
}

// PubSubList 当前订阅状态（订阅由后端通道接收，前端自行记录频道清单）。
func (m *Manager) PubSubList(connID string) ([]string, []string, error) {
	b, err := m.Get(connID)
	if err != nil {
		return nil, nil, err
	}
	_ = b
	return nil, nil, nil
}

// IsActive 订阅会话是否活跃。
func (p *PubSubManager) IsActive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.active
}
