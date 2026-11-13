package rds

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/redis/go-redis/v9"

	"RedisLens/internal/config"
)

// NodeInfo 拓扑节点。
type NodeInfo struct {
	ID    string `json:"id,omitempty"`
	Addr  string `json:"addr"`
	Role  string `json:"role"` // master | replica | sentinel
	Slots string `json:"slots,omitempty"`
	State string `json:"state,omitempty"`
}

// Topology 拓扑视图。
type Topology struct {
	Mode   string     `json:"mode"`
	Master string     `json:"master,omitempty"`
	Nodes  []NodeInfo `json:"nodes"`
	Shards int        `json:"shards,omitempty"`
}

// GetTopology 自动发现并返回实例拓扑（standalone / sentinel / cluster）。
func (m *Manager) GetTopology(connID string) (*Topology, error) {
	b, err := m.Get(connID)
	if err != nil {
		return nil, err
	}
	cli := b.Client
	if cli == nil {
		return nil, fmt.Errorf("connection is down")
	}
	ctx, cancel := opCtx()
	defer cancel()

	switch b.Cfg.Mode {
	case "cluster":
		return clusterTopology(ctx, cli)
	case "sentinel":
		return sentinelTopology(ctx, &b.Cfg)
	default:
		// 兼容低版本：全量 INFO 后自行解析 replication 段
		info, err := cli.Info(ctx).Result()
		if err != nil {
			return nil, err
		}
		kv := parseInfo(info)
		topo := &Topology{Mode: "standalone", Master: encodeTarget(b.Cfg.Host, b.Cfg.Port)}
		topo.Nodes = append(topo.Nodes, NodeInfo{Addr: topo.Master, Role: kv["role"]})
		slaves := int(parseInt(kv["connected_slaves"]))
		for i := 0; i < slaves; i++ {
			// slave0:ip=127.0.0.1,port=6380,state=online,offset=xxx,lag=0
			fields := parseSlaveLine(kv[fmt.Sprintf("slave%d", i)])
			topo.Nodes = append(topo.Nodes, NodeInfo{
				Addr:  fmt.Sprintf("%s:%s", fields["ip"], fields["port"]),
				Role:  "replica",
				State: fields["state"],
			})
		}
		return topo, nil
	}
}

func parseSlaveLine(line string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(line, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

// sentinelTopology 通过哨兵协议发现主从拓扑。
func sentinelTopology(ctx context.Context, cfg *config.Connection) (*Topology, error) {
	topo := &Topology{Mode: "sentinel"}
	for _, addr := range cfg.Sentinel.Addrs {
		sc := redis.NewSentinelClient(&redis.Options{
			Addr:        addr,
			Username:    cfg.Sentinel.Username,
			Password:    cfg.Sentinel.Password,
			DialTimeout: dialTimeout,
			ReadTimeout: readTimeout,
		})
		masterAddr, err := sc.GetMasterAddrByName(ctx, cfg.Sentinel.Master).Result()
		if err != nil {
			_ = sc.Close()
			continue
		}
		if len(masterAddr) >= 2 {
			topo.Master = fmt.Sprintf("%s:%s", masterAddr[0], masterAddr[1])
			topo.Nodes = append(topo.Nodes, NodeInfo{Addr: topo.Master, Role: "master"})
		}
		topo.Nodes = append(topo.Nodes, NodeInfo{Addr: addr, Role: "sentinel"})
		// 查询副本
		if reps, err := sc.Replicas(ctx, cfg.Sentinel.Master).Result(); err == nil {
			for _, rep := range reps {
				ip := rep["ip"]
				port := rep["port"]
				state := rep["master-link-status"]
				if ip != "" && port != "" {
					topo.Nodes = append(topo.Nodes, NodeInfo{
						Addr:  fmt.Sprintf("%s:%s", ip, port),
						Role:  "replica",
						State: state,
					})
				}
			}
		}
		_ = sc.Close()
		topo.Shards = 1
		return topo, nil
	}
	return nil, fmt.Errorf("无法连接任何哨兵节点")
}

// clusterTopology 从 CLUSTER SLOTS 构建分片拓扑。
func clusterTopology(ctx context.Context, cli redis.UniversalClient) (*Topology, error) {
	cc, ok := cli.(*redis.ClusterClient)
	if !ok {
		return nil, fmt.Errorf("not a cluster client")
	}
	topo := &Topology{Mode: "cluster"}
	slots, err := cc.ClusterSlots(ctx).Result()
	if err != nil {
		return nil, err
	}
	for _, s := range slots {
		slotRange := fmt.Sprintf("%d-%d", s.Start, s.End)
		for _, n := range s.Nodes {
			role := "replica"
			if len(s.Nodes) > 0 && n.Addr == s.Nodes[0].Addr {
				role = "master" // go-redis 返回的第一个节点为主节点
			}
			topo.Nodes = append(topo.Nodes, NodeInfo{
				ID:    n.ID,
				Addr:  n.Addr,
				Role:  role,
				Slots: slotRange,
			})
		}
		topo.Shards++
	}
	sort.Slice(topo.Nodes, func(i, j int) bool {
		if topo.Nodes[i].Role != topo.Nodes[j].Role {
			return topo.Nodes[i].Role == "master"
		}
		return topo.Nodes[i].Addr < topo.Nodes[j].Addr
	})
	return topo, nil
}
