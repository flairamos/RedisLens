export namespace audit {
	
	export class Entry {
	    // Go type: time
	    time: any;
	    connId: string;
	    connName: string;
	    category: string;
	    action: string;
	    target: string;
	    detail?: string;
	    result: string;
	    level: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = this.convertValues(source["time"], null);
	        this.connId = source["connId"];
	        this.connName = source["connName"];
	        this.category = source["category"];
	        this.action = source["action"];
	        this.target = source["target"];
	        this.detail = source["detail"];
	        this.result = source["result"];
	        this.level = source["level"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace config {
	
	export class SentinelConfig {
	    master: string;
	    addrs: string[];
	    username?: string;
	    password?: string;
	
	    static createFrom(source: any = {}) {
	        return new SentinelConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.master = source["master"];
	        this.addrs = source["addrs"];
	        this.username = source["username"];
	        this.password = source["password"];
	    }
	}
	export class TLSConfig {
	    enabled: boolean;
	    certFile?: string;
	    keyFile?: string;
	    caFile?: string;
	    serverName?: string;
	    insecureSkipVerify?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TLSConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.certFile = source["certFile"];
	        this.keyFile = source["keyFile"];
	        this.caFile = source["caFile"];
	        this.serverName = source["serverName"];
	        this.insecureSkipVerify = source["insecureSkipVerify"];
	    }
	}
	export class SSHConfig {
	    enabled: boolean;
	    host: string;
	    port: number;
	    user: string;
	    authType: string;
	    password?: string;
	    keyPath?: string;
	    passphrase?: string;
	
	    static createFrom(source: any = {}) {
	        return new SSHConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.authType = source["authType"];
	        this.password = source["password"];
	        this.keyPath = source["keyPath"];
	        this.passphrase = source["passphrase"];
	    }
	}
	export class Connection {
	    id: string;
	    name: string;
	    group: string;
	    mode: string;
	    host: string;
	    port: number;
	    username?: string;
	    db: number;
	    readOnly: boolean;
	    ssh: SSHConfig;
	    tls: TLSConfig;
	    sentinel?: SentinelConfig;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    lastUsedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.group = source["group"];
	        this.mode = source["mode"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.db = source["db"];
	        this.readOnly = source["readOnly"];
	        this.ssh = this.convertValues(source["ssh"], SSHConfig);
	        this.tls = this.convertValues(source["tls"], TLSConfig);
	        this.sentinel = this.convertValues(source["sentinel"], SentinelConfig);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.lastUsedAt = this.convertValues(source["lastUsedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class Settings {
	    theme: string;
	    density: string;
	    monitorIntervalSec: number;
	    confirmDanger: boolean;
	    logEnabled: boolean;
	    scanCount: number;
	    valuePreviewLimit: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.density = source["density"];
	        this.monitorIntervalSec = source["monitorIntervalSec"];
	        this.confirmDanger = source["confirmDanger"];
	        this.logEnabled = source["logEnabled"];
	        this.scanCount = source["scanCount"];
	        this.valuePreviewLimit = source["valuePreviewLimit"];
	    }
	}

}

export namespace main {
	
	export class connDTO {
	    id: string;
	    name: string;
	    group: string;
	    mode: string;
	    host: string;
	    port: number;
	    username?: string;
	    db: number;
	    readOnly: boolean;
	    ssh: config.SSHConfig;
	    tls: config.TLSConfig;
	    sentinel?: config.SentinelConfig;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    lastUsedAt: any;
	    hasPassword: boolean;
	    hasSSHPass: boolean;
	    hasSSHKey: boolean;
	    health: string;
	    healthError: string;
	    open: boolean;
	
	    static createFrom(source: any = {}) {
	        return new connDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.group = source["group"];
	        this.mode = source["mode"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.db = source["db"];
	        this.readOnly = source["readOnly"];
	        this.ssh = this.convertValues(source["ssh"], config.SSHConfig);
	        this.tls = this.convertValues(source["tls"], config.TLSConfig);
	        this.sentinel = this.convertValues(source["sentinel"], config.SentinelConfig);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.lastUsedAt = this.convertValues(source["lastUsedAt"], null);
	        this.hasPassword = source["hasPassword"];
	        this.hasSSHPass = source["hasSSHPass"];
	        this.hasSSHKey = source["hasSSHKey"];
	        this.health = source["health"];
	        this.healthError = source["healthError"];
	        this.open = source["open"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace rds {
	
	export class CmdStat {
	    command: string;
	    calls: number;
	    usec: number;
	
	    static createFrom(source: any = {}) {
	        return new CmdStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.command = source["command"];
	        this.calls = source["calls"];
	        this.usec = source["usec"];
	    }
	}
	export class CommandHelp {
	    name: string;
	    group: string;
	    synop: string;
	    danger: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CommandHelp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.group = source["group"];
	        this.synop = source["synop"];
	        this.danger = source["danger"];
	    }
	}
	export class ConsoleResult {
	    ok: boolean;
	    blocked: boolean;
	    needConfirm: boolean;
	    reason?: string;
	    op?: string;
	    reply: string;
	    rows?: string[];
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new ConsoleResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.blocked = source["blocked"];
	        this.needConfirm = source["needConfirm"];
	        this.reason = source["reason"];
	        this.op = source["op"];
	        this.reply = source["reply"];
	        this.rows = source["rows"];
	        this.duration = source["duration"];
	    }
	}
	export class KeyItem {
	    key: string;
	    type: string;
	    ttl: number;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new KeyItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.type = source["type"];
	        this.ttl = source["ttl"];
	        this.size = source["size"];
	    }
	}
	export class DeletePreview {
	    match: string;
	    total: number;
	    samples: KeyItem[];
	
	    static createFrom(source: any = {}) {
	        return new DeletePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.match = source["match"];
	        this.total = source["total"];
	        this.samples = this.convertValues(source["samples"], KeyItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DeleteResult {
	    deleted: number;
	    backedUp: number;
	    tokens: string[];
	
	    static createFrom(source: any = {}) {
	        return new DeleteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deleted = source["deleted"];
	        this.backedUp = source["backedUp"];
	        this.tokens = source["tokens"];
	    }
	}
	export class ExportReport {
	    file: string;
	    keys: number;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt: any;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new ExportReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.keys = source["keys"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
	        this.durationMs = source["durationMs"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FieldItem {
	    field: string;
	    value: string;
	    binary: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FieldItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.field = source["field"];
	        this.value = source["value"];
	        this.binary = source["binary"];
	    }
	}
	export class ImportReport {
	    file: string;
	    total: number;
	    imported: number;
	    skipped: number;
	    failed: number;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt: any;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.total = source["total"];
	        this.imported = source["imported"];
	        this.skipped = source["skipped"];
	        this.failed = source["failed"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
	        this.durationMs = source["durationMs"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ListItem {
	    index: number;
	    value: string;
	    binary: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ListItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.value = source["value"];
	        this.binary = source["binary"];
	    }
	}
	export class NodeInfo {
	    id?: string;
	    addr: string;
	    role: string;
	    slots?: string;
	    state?: string;
	
	    static createFrom(source: any = {}) {
	        return new NodeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.addr = source["addr"];
	        this.role = source["role"];
	        this.slots = source["slots"];
	        this.state = source["state"];
	    }
	}
	export class RecycleItem {
	    token: string;
	    key: string;
	    type: string;
	    pttl: number;
	    // Go type: time
	    deletedAt: any;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new RecycleItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.key = source["key"];
	        this.type = source["type"];
	        this.pttl = source["pttl"];
	        this.deletedAt = this.convertValues(source["deletedAt"], null);
	        this.size = source["size"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Sample {
	    time: number;
	    qps: number;
	    hitRate: number;
	    hitsRate: number;
	    missRate: number;
	    usedMem: number;
	    maxMem: number;
	    rssMem: number;
	    fragRatio: number;
	    clients: number;
	    blocked: number;
	    netInRate: number;
	    netOutRate: number;
	    totalKeys: number;
	    expiredRate: number;
	    evictedRate: number;
	    cpuUserRate: number;
	    cpuSysRate: number;
	    role: string;
	    slaves: number;
	    replLag: number;
	    masterLink: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new Sample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.qps = source["qps"];
	        this.hitRate = source["hitRate"];
	        this.hitsRate = source["hitsRate"];
	        this.missRate = source["missRate"];
	        this.usedMem = source["usedMem"];
	        this.maxMem = source["maxMem"];
	        this.rssMem = source["rssMem"];
	        this.fragRatio = source["fragRatio"];
	        this.clients = source["clients"];
	        this.blocked = source["blocked"];
	        this.netInRate = source["netInRate"];
	        this.netOutRate = source["netOutRate"];
	        this.totalKeys = source["totalKeys"];
	        this.expiredRate = source["expiredRate"];
	        this.evictedRate = source["evictedRate"];
	        this.cpuUserRate = source["cpuUserRate"];
	        this.cpuSysRate = source["cpuSysRate"];
	        this.role = source["role"];
	        this.slaves = source["slaves"];
	        this.replLag = source["replLag"];
	        this.masterLink = source["masterLink"];
	        this.version = source["version"];
	    }
	}
	export class ScanPageResp {
	    cursor: number;
	    keys: KeyItem[];
	    done: boolean;
	    scanned: number;
	
	    static createFrom(source: any = {}) {
	        return new ScanPageResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cursor = source["cursor"];
	        this.keys = this.convertValues(source["keys"], KeyItem);
	        this.done = source["done"];
	        this.scanned = source["scanned"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ServerSummary {
	    version: string;
	    mode: string;
	    role: string;
	    os: string;
	    usedMem: number;
	    totalKeys: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.mode = source["mode"];
	        this.role = source["role"];
	        this.os = source["os"];
	        this.usedMem = source["usedMem"];
	        this.totalKeys = source["totalKeys"];
	    }
	}
	export class SlowLogEntry {
	    id: number;
	    time: number;
	    duration: number;
	    command: string;
	    client: string;
	    clientName: string;
	
	    static createFrom(source: any = {}) {
	        return new SlowLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = source["time"];
	        this.duration = source["duration"];
	        this.command = source["command"];
	        this.client = source["client"];
	        this.clientName = source["clientName"];
	    }
	}
	export class StreamEntry {
	    id: string;
	    fields: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new StreamEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.fields = source["fields"];
	    }
	}
	export class TTLResult {
	    affected: number;
	
	    static createFrom(source: any = {}) {
	        return new TTLResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.affected = source["affected"];
	    }
	}
	export class Topology {
	    mode: string;
	    master?: string;
	    nodes: NodeInfo[];
	    shards?: number;
	
	    static createFrom(source: any = {}) {
	        return new Topology(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.master = source["master"];
	        this.nodes = this.convertValues(source["nodes"], NodeInfo);
	        this.shards = source["shards"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TreeNode {
	    name: string;
	    prefix: string;
	    isFolder: boolean;
	    type?: string;
	    ttl?: number;
	
	    static createFrom(source: any = {}) {
	        return new TreeNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.prefix = source["prefix"];
	        this.isFolder = source["isFolder"];
	        this.type = source["type"];
	        this.ttl = source["ttl"];
	    }
	}
	export class ZSetItem {
	    rank: number;
	    member: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new ZSetItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rank = source["rank"];
	        this.member = source["member"];
	        this.score = source["score"];
	    }
	}
	export class ValueData {
	    key: string;
	    type: string;
	    encoding?: string;
	    ttl: number;
	    size: number;
	    elements: number;
	    binary: boolean;
	    str?: string;
	    isJson?: boolean;
	    hash?: FieldItem[];
	    list?: ListItem[];
	    set?: string[];
	    zset?: ZSetItem[];
	    stream?: StreamEntry[];
	
	    static createFrom(source: any = {}) {
	        return new ValueData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.type = source["type"];
	        this.encoding = source["encoding"];
	        this.ttl = source["ttl"];
	        this.size = source["size"];
	        this.elements = source["elements"];
	        this.binary = source["binary"];
	        this.str = source["str"];
	        this.isJson = source["isJson"];
	        this.hash = this.convertValues(source["hash"], FieldItem);
	        this.list = this.convertValues(source["list"], ListItem);
	        this.set = source["set"];
	        this.zset = this.convertValues(source["zset"], ZSetItem);
	        this.stream = this.convertValues(source["stream"], StreamEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

