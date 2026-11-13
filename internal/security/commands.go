// Package security 定义 Redis 命令分类：读 / 写 / 危险，用于
// 只读模式拦截、危险命令二次确认与控制台补全。覆盖 Redis 4.0 - 7.x 常用命令。
package security

import (
	"strings"
)

// Class 命令类别。
type Class int

const (
	ClassUnknown    Class = iota // 未知命令（按写处理，保守策略）
	ClassRead                    // 只读命令
	ClassWrite                   // 写命令
	ClassDangerous               // 危险命令：需二次确认并审计
	ClassStreaming               // 流式/阻塞命令（MONITOR 等），默认拦截确认
)

// String 类别名称。
func (c Class) String() string {
	switch c {
	case ClassRead:
		return "read"
	case ClassWrite:
		return "write"
	case ClassDangerous:
		return "dangerous"
	case ClassStreaming:
		return "streaming"
	default:
		return "unknown"
	}
}

// Cmd 命令元数据。
type Cmd struct {
	Name   string // 大写规范名
	Group  string // 分组：string/hash/list/set/zset/stream/key/server/generic/pubsub/cluster/script/admin
	Synop  string // 一句话中文说明（用于补全提示）
	Class  Class  // 默认类别
	ReadOnly bool // 是否只读命令
}

// subClass 子命令覆写表：父命令 -> 子命令 -> 类别。未命中的子命令继承父命令类别。
var subClass = map[string]map[string]Class{
	"CONFIG":   {"GET": ClassRead, "RESETSTAT": ClassDangerous, "REWRITE": ClassWrite, "SET": ClassDangerous, "HELP": ClassRead},
	"DEBUG":    {"JMAP": ClassRead, "OBJECT": ClassRead, "SEGINFO": ClassRead, "QUICKLIST-PACKED": ClassRead},
	"CLIENT":   {"LIST": ClassRead, "ID": ClassRead, "INFO": ClassRead, "GETNAME": ClassRead, "NO-EVICT": ClassRead, "REPLY": ClassRead, "TRACKING": ClassWrite, "CACHING": ClassWrite, "SETNAME": ClassWrite, "UNPAUSE": ClassDangerous, "PAUSE": ClassDangerous, "KILL": ClassDangerous},
	"SCRIPT":   {"EXISTS": ClassRead, "LOAD": ClassWrite, "FLUSH": ClassDangerous},
	"FUNCTION": {"LIST": ClassRead, "STATS": ClassRead, "DUMP": ClassRead, "LOAD": ClassDangerous, "DELETE": ClassDangerous, "FLUSH": ClassDangerous, "RESTORE": ClassDangerous},
	"CLUSTER":  {"INFO": ClassRead, "NODES": ClassRead, "MYID": ClassRead, "SLOTS": ClassRead, "SHARDS": ClassRead, "SLAVES": ClassRead, "REPLICAS": ClassRead, "KEYSLOT": ClassRead, "COUNTKEYSINSLOT": ClassRead, "GETKEYSINSLOT": ClassRead, "COUNT-FAILURE-REPORTS": ClassRead, "COUNT-FAILING-NODES": ClassRead, "LINKS": ClassRead, "RESET": ClassDangerous, "ADDSLOTS": ClassDangerous, "DELSLOTS": ClassDangerous, "ADDSLOTSRANGE": ClassDangerous, "DELSLOTSRANGE": ClassDangerous, "SETSLOT": ClassDangerous, "BUMPEPOCH": ClassDangerous, "FAILOVER": ClassDangerous, "REPLICATE": ClassDangerous, "FORGET": ClassDangerous},
	"ACL":      {"LIST": ClassRead, "WHOAMI": ClassRead, "GETUSER": ClassRead, "USERS": ClassRead, "CAT": ClassRead, "GENPASS": ClassRead, "LOG": ClassRead, "DRYRUN": ClassRead, "SETUSER": ClassDangerous, "DELUSER": ClassDangerous, "SAVE": ClassWrite},
	"MEMORY":   {"USAGE": ClassRead, "STATS": ClassRead, "DOCTOR": ClassRead, "MALLOC-STATS": ClassRead, "PURGE": ClassWrite},
	"SLOWLOG":  {"GET": ClassRead, "LEN": ClassRead, "RESET": ClassDangerous},
	"XGROUP":   {"CREATE": ClassWrite, "CREATECONSUMER": ClassWrite, "DELCONSUMER": ClassDangerous, "DESTROY": ClassDangerous, "SETID": ClassWrite},
	"OBJECT":   {"HELP": ClassRead, "ENCODING": ClassRead, "FREQ": ClassRead, "IDLETIME": ClassRead, "REFCOUNT": ClassRead},
	"LATENCY":  {"HISTORY": ClassRead, "LATEST": ClassRead, "RESET": ClassDangerous, "GRAPH": ClassRead, "DOCTOR": ClassRead},
}

// registry 命令注册表。
var registry = map[string]*Cmd{}

// register 批量注册。
func register(class Class, group string, synop string, names ...string) {
	for _, n := range names {
		registry[n] = &Cmd{Name: n, Group: group, Synop: synop, Class: class, ReadOnly: class == ClassRead}
	}
}

func init() {
	// ---- 键（generic）----
	register(ClassRead, "key", "列出匹配模式的键（阻塞风险，受管控）", "KEYS")
	registry["KEYS"].Class = ClassStreaming // KEYS 全量扫描按阻塞命令管控
	register(ClassRead, "key", "游标渐进遍历键", "SCAN")
	register(ClassRead, "key", "随机返回一个键", "RANDOMKEY")
	register(ClassWrite, "key", "删除键", "DEL", "UNLINK")
	register(ClassWrite, "key", "判断键是否存在", "EXISTS")
	register(ClassRead, "key", "查询键类型", "TYPE")
	register(ClassRead, "key", "返回键剩余存活时间（秒）", "TTL", "PTTL")
	register(ClassWrite, "key", "设置键过期时间", "EXPIRE", "PEXPIRE", "EXPIREAT", "PEXPIREAT", "EXPIRETIME")
	register(ClassWrite, "key", "移除键的过期时间", "PERSIST")
	register(ClassWrite, "key", "重命名键", "RENAME", "RENAMENX")
	register(ClassRead, "key", "序列化键值", "DUMP")
	register(ClassDangerous, "key", "反序列化恢复键", "RESTORE")
	register(ClassRead, "key", "返回键内部编码", "OBJECT")

	// ---- 字符串 ----
	register(ClassRead, "string", "读取字符串键", "GET", "MGET", "GETRANGE", "STRLEN", "GETDEL")
	registry["GETDEL"].Class = ClassWrite
	registry["GETDEL"].ReadOnly = false
	register(ClassWrite, "string", "写入字符串键", "SET", "MSET", "SETNX", "MSETNX", "SETBIT", "SETRANGE", "SETEX", "PSETEX", "GETSET", "SETNX", "APPEND")
	register(ClassWrite, "string", "计数器增减", "INCR", "INCRBY", "INCRBYFLOAT", "DECR", "DECRBY")
	register(ClassRead, "string", "位图统计", "BITCOUNT", "BITPOS", "BITFIELD", "BITFIELD_RO", "GETBIT")

	// ---- 哈希 ----
	register(ClassRead, "hash", "读取哈希字段", "HGET", "HMGET", "HGETALL", "HKEYS", "HVALS", "HLEN", "HEXISTS", "HSTRLEN", "HSCAN", "HRANDFIELD", "HGETDEL", "HGETEX")
	registry["HGETDEL"].Class = ClassWrite
	registry["HGETDEL"].ReadOnly = false
	register(ClassWrite, "hash", "写入哈希字段", "HSET", "HMSET", "HSETNX", "HDEL", "HINCRBY", "HINCRBYFLOAT")

	// ---- 列表 ----
	register(ClassRead, "list", "读取列表元素", "LRANGE", "LLEN", "LINDEX", "LPOS")
	register(ClassWrite, "list", "写入列表元素", "LPUSH", "RPUSH", "LPUSHX", "RPUSHX", "LPOP", "RPOP", "LSET", "LINSERT", "LREM", "LTRIM", "LMOVE", "LMPOP", "BLMOVE", "BLMPOP", "BLPOP", "BRPOP", "BRPOPLPUSH")

	// ---- 集合 ----
	register(ClassRead, "set", "读取集合成员", "SMEMBERS", "SCARD", "SISMEMBER", "SMISMEMBER", "SRANDMEMBER", "SSCAN", "SINTERCARD")
	register(ClassWrite, "set", "写入集合成员", "SADD", "SREM", "SPOP", "SMOVE", "SINTERSTORE", "SUNIONSTORE", "SDIFFSTORE")

	// ---- 有序集合 ----
	register(ClassRead, "zset", "读取有序集合", "ZRANGE", "ZRANGEBYSCORE", "ZRANGEBYLEX", "ZREVRANGE", "ZREVRANGEBYSCORE", "ZREVRANGEBYLEX", "ZCARD", "ZCOUNT", "ZSCORE", "ZMSCORE", "ZRANK", "ZREVRANK", "ZSCAN", "ZRANDMEMBER", "ZLEXCOUNT")
	register(ClassWrite, "zset", "写入有序集合", "ZADD", "ZINCRBY", "ZREM", "ZREMRANGEBYRANK", "ZREMRANGEBYSCORE", "ZREMRANGEBYLEX", "ZPOPMIN", "ZPOPMAX", "ZMPOP", "BZPOPMIN", "BZPOPMAX", "BZMPOP", "ZRANGESTORE")

	// ---- Stream ----
	register(ClassRead, "stream", "读取流消息", "XLEN", "XRANGE", "XREVRANGE", "XREAD", "XPENDING", "XINFO")
	registry["XINFO"].Class = ClassRead
	register(ClassWrite, "stream", "写入流消息", "XADD", "XDEL", "XTRIM", "XACK", "XCLAIM", "XAUTOCLAIM", "XSETID", "XREADGROUP")

	// ---- HyperLogLog / 位图 ----
	register(ClassRead, "hyperloglog", "基数估计", "PFCOUNT", "PFMERGE")
	registry["PFMERGE"].Class = ClassWrite
	registry["PFMERGE"].ReadOnly = false
	register(ClassWrite, "hyperloglog", "添加基数估计元素", "PFADD")
	register(ClassRead, "bitmap", "位图读取", "BITPOS")
	register(ClassWrite, "bitmap", "位图写入", "SETBIT")
	registry["SETBIT"].Group = "string"

	// ---- 通用服务端 ----
	register(ClassRead, "server", "查询服务端信息", "INFO", "DBSIZE", "COMMAND", "TIME", "LOLWUT", "PING", "ECHO", "SELECT", "LASTSAVE", "ROLE", "MEMORY", "LATENCY", "CLIENT", "ACL")
	registry["CLIENT"].Class = ClassRead
	// CONFIG 基类危险（SET/REWRITE/RESETSTAT），GET/HELP 由子命令覆写放行
	register(ClassDangerous, "server", "服务端配置（危险）", "CONFIG")
	register(ClassDangerous, "server", "清空当前库（危险）", "FLUSHDB")
	register(ClassDangerous, "server", "清空所有库（危险）", "FLUSHALL")
	register(ClassDangerous, "server", "调试命令（危险）", "DEBUG")
	register(ClassDangerous, "server", "安全关闭实例（危险）", "SHUTDOWN")
	register(ClassDangerous, "server", "设置主从复制（危险）", "SLAVEOF", "REPLICAOF")
	register(ClassDangerous, "server", "交换数据库（危险）", "SWAPDB")
	register(ClassDangerous, "server", "跨实例迁移键（危险）", "MIGRATE")
	register(ClassStreaming, "server", "实时监视命令流（阻塞）", "MONITOR")
	register(ClassWrite, "server", "保存数据", "SAVE", "BGSAVE", "BGREWRITEAOF", "WAIT", "WAITAOF")
	register(ClassWrite, "server", "切换数据库", "MOVE", "COPY")
	registry["COPY"].Class = ClassWrite
	registry["MOVE"].Class = ClassWrite

	// ---- 脚本 / 函数 ----
	register(ClassRead, "script", "执行脚本（可能写数据）", "EVAL", "EVALSHA", "EVAL_RO", "EVALSHA_RO", "FCALL", "FCALL_RO", "SCRIPT", "FUNCTION")
	registry["EVAL"].Class = ClassWrite
	registry["EVALSHA"].Class = ClassWrite
	registry["FCALL"].Class = ClassWrite
	registry["EVAL_RO"].Class = ClassRead
	registry["EVALSHA_RO"].Class = ClassRead
	registry["FCALL_RO"].Class = ClassRead

	// ---- 发布订阅 ----
	register(ClassRead, "pubsub", "发布订阅", "PUBSUB", "PUBLISH", "SPUBLISH", "SSUBSCRIBE", "SUBSCRIBE", "UNSUBSCRIBE", "SUNSUBSCRIBE", "PSUBSCRIBE", "PUNSUBSCRIBE")
	registry["PUBLISH"].Class = ClassWrite
	registry["SPUBLISH"].Class = ClassWrite
	registry["SSUBSCRIBE"].Class = ClassStreaming
	registry["SUBSCRIBE"].Class = ClassStreaming
	registry["PSUBSCRIBE"].Class = ClassStreaming
	registry["UNSUBSCRIBE"].Class = ClassRead
	registry["PUNSUBSCRIBE"].Class = ClassRead
	registry["SUNSUBSCRIBE"].Class = ClassRead

	// ---- 事务 ----
	register(ClassRead, "transaction", "事务", "MULTI", "EXEC", "DISCARD", "WATCH", "UNWATCH")
	registry["MULTI"].Class = ClassWrite
	registry["EXEC"].Class = ClassWrite
	registry["DISCARD"].Class = ClassRead

	// ---- Geo ----
	register(ClassRead, "geo", "地理位置读取", "GEOPOS", "GEODIST", "GEOHASH", "GEOSEARCH", "GEOSEARCHSTORE")
	registry["GEOSEARCHSTORE"].Class = ClassWrite
	registry["GEOSEARCHSTORE"].ReadOnly = false
	register(ClassWrite, "geo", "地理位置写入", "GEOADD")

	// ---- 集群 ----
	register(ClassRead, "cluster", "集群查询", "CLUSTER")
	register(ClassWrite, "cluster", "集群键迁移", "ASKING", "READONLY", "READWRITE")

	// ---- 逐出淘汰 ----
	register(ClassRead, "server", "慢查询日志", "SLOWLOG")
	register(ClassRead, "server", "列出连接的客户端", "CLIENT")
}

// ParseCommand 将一行控制台输入拆分为参数列表（支持单双引号与转义）。
func ParseCommand(line string) []string {
	var args []string
	var cur strings.Builder
	inSingle, inDouble, hasToken := false, false, false
	flush := func() {
		if hasToken {
			args = append(args, cur.String())
			cur.Reset()
			hasToken = false
		}
	}
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case inSingle:
			if r == '\'' {
				inSingle = false
			} else {
				cur.WriteRune(r)
			}
		case inDouble:
			if r == '"' {
				inDouble = false
			} else if r == '\\' && i+1 < len(rs) {
				i++
				n := rs[i]
				switch n {
				case 'n':
					cur.WriteRune('\n')
				case 't':
					cur.WriteRune('\t')
				case 'r':
					cur.WriteRune('\r')
				default:
					cur.WriteRune(n)
				}
			} else {
				cur.WriteRune(r)
			}
		case r == '\'':
			inSingle = true
			hasToken = true
		case r == '"':
			inDouble = true
			hasToken = true
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		case r == '\\' && i+1 < len(rs):
			i++
			cur.WriteRune(rs[i])
			hasToken = true
		default:
			cur.WriteRune(r)
			hasToken = true
		}
	}
	flush()
	return args
}

// Classify 返回命令的类别。优先应用子命令覆写。
func Classify(args []string) (Class, *Cmd) {
	if len(args) == 0 {
		return ClassUnknown, nil
	}
	name := strings.ToUpper(args[0])
	cmd, ok := registry[name]
	if !ok {
		return ClassWrite, nil // 未知命令按写处理（只读模式拦截）
	}
	class := cmd.Class
	if len(args) > 1 {
		if subs, has := subClass[name]; has {
			sub := strings.ToUpper(args[1])
			// 子命令可能带额外 token，取第一个词
			if sc, found := subs[sub]; found {
				class = sc
			}
		}
	}
	return class, cmd
}

// IsReadOnlyCommand 判断命令是否为只读（只读模式下放行）。
func IsReadOnlyCommand(args []string) bool {
	class, cmd := Classify(args)
	if cmd != nil && cmd.ReadOnly && class == ClassRead {
		return true
	}
	return class == ClassRead
}

// DangerousCommands 返回危险命令名集合（用于 UI 提示）。
func DangerousCommands() []string {
	var out []string
	for name, cmd := range registry {
		if cmd.Class == ClassDangerous || cmd.Class == ClassStreaming {
			out = append(out, name)
		}
	}
	return out
}

// Registry 暴露命令注册表（用于控制台补全）。
func Registry() map[string]*Cmd { return registry }
