export namespace diag {
	
	export class Item {
	    name: string;
	    ok: boolean;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ok = source["ok"];
	        this.detail = source["detail"];
	    }
	}
	export class Report {
	    local: Item[];
	    server: Item[];
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.local = this.convertValues(source["local"], Item);
	        this.server = this.convertValues(source["server"], Item);
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

export namespace main {
	
	export class StatusInfo {
	    running: boolean;
	    connected: boolean;
	    ai_ready: boolean;
	    phys_ip: string;
	    last_action: string;
	    session_min: number;
	    active_decoy: string;
	    adaptive_enabled: boolean;
	    adaptive_mode: string;
	    adaptive_session_min: number;
	    adaptive_url_count: number;
	    adaptive_chain_depth: number;
	    adaptive_site_cap: number;
	    adaptive_rag_weight: number;
	    adaptive_timeout_policy: string;
	
	    static createFrom(source: any = {}) {
	        return new StatusInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.connected = source["connected"];
	        this.ai_ready = source["ai_ready"];
	        this.phys_ip = source["phys_ip"];
	        this.last_action = source["last_action"];
	        this.session_min = source["session_min"];
	        this.active_decoy = source["active_decoy"];
	        this.adaptive_enabled = source["adaptive_enabled"];
	        this.adaptive_mode = source["adaptive_mode"];
	        this.adaptive_session_min = source["adaptive_session_min"];
	        this.adaptive_url_count = source["adaptive_url_count"];
	        this.adaptive_chain_depth = source["adaptive_chain_depth"];
	        this.adaptive_site_cap = source["adaptive_site_cap"];
	        this.adaptive_rag_weight = source["adaptive_rag_weight"];
	        this.adaptive_timeout_policy = source["adaptive_timeout_policy"];
	    }
	}

}

export namespace memory {
	
	export class Action {
	    cover_urls: string[];
	    timing_ms: number;
	    hotswap: boolean;
	    decoy_domain: string;
	
	    static createFrom(source: any = {}) {
	        return new Action(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cover_urls = source["cover_urls"];
	        this.timing_ms = source["timing_ms"];
	        this.hotswap = source["hotswap"];
	        this.decoy_domain = source["decoy_domain"];
	    }
	}
	export class Context {
	    hour: number;
	    day_type: string;
	    rtt_trend: string;
	    profile: string;
	    intensity: string;
	
	    static createFrom(source: any = {}) {
	        return new Context(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hour = source["hour"];
	        this.day_type = source["day_type"];
	        this.rtt_trend = source["rtt_trend"];
	        this.profile = source["profile"];
	        this.intensity = source["intensity"];
	    }
	}
	export class Entry {
	    id: string;
	    // Go type: time
	    timestamp: any;
	    context: Context;
	    action: Action;
	    outcome: string;
	    score: number;
	    rtt_before_ms: number;
	    rtt_after_ms: number;
	    eval_timed_out: boolean;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.context = this.convertValues(source["context"], Context);
	        this.action = this.convertValues(source["action"], Action);
	        this.outcome = source["outcome"];
	        this.score = source["score"];
	        this.rtt_before_ms = source["rtt_before_ms"];
	        this.rtt_after_ms = source["rtt_after_ms"];
	        this.eval_timed_out = source["eval_timed_out"];
	        this.note = source["note"];
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
	export class Stats {
	    total: number;
	    successes: number;
	    failures: number;
	    timeouts: number;
	    avg_score: number;
	    oldest_days: number;
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.successes = source["successes"];
	        this.failures = source["failures"];
	        this.timeouts = source["timeouts"];
	        this.avg_score = source["avg_score"];
	        this.oldest_days = source["oldest_days"];
	    }
	}

}

export namespace profile {
	
	export class ClientSummary {
	    id: string;
	    name: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ClientSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.active = source["active"];
	    }
	}
	export class CoverList {
	    id: string;
	    name: string;
	    sites: string[];
	
	    static createFrom(source: any = {}) {
	        return new CoverList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.sites = source["sites"];
	    }
	}
	export class Profile {
	    id: string;
	    name: string;
	    server_id: string;
	    server_name: string;
	    client_id: string;
	    client_name: string;
	    vps_host: string;
	    vps_port: number;
	    vps_user: string;
	    vps_password: string;
	    deployed: boolean;
	    awg_listen_port: string;
	    awg_client_priv_key: string;
	    awg_client_pub_key: string;
	    awg_server_pub_key: string;
	    awg_client_config: string;
	    awg_exe_path: string;
	    awg_interface: string;
	    active_decoy_domain: string;
	    decoy_domains: string[];
	    hotswap_enabled: boolean;
	    hotswap_interval_min: number;
	    lmstudio_model: string;
	    ai_system_prompt: string;
	    cover_sites: string[];
	    behavior_profile: string;
	    cover_lists: CoverList[];
	    active_cover_list_id: string;
	    intensity: string;
	    session_minutes: number;
	    memory_timeout_policy: string;
	    adaptive_control: boolean;
	    adaptive_mode: string;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.server_id = source["server_id"];
	        this.server_name = source["server_name"];
	        this.client_id = source["client_id"];
	        this.client_name = source["client_name"];
	        this.vps_host = source["vps_host"];
	        this.vps_port = source["vps_port"];
	        this.vps_user = source["vps_user"];
	        this.vps_password = source["vps_password"];
	        this.deployed = source["deployed"];
	        this.awg_listen_port = source["awg_listen_port"];
	        this.awg_client_priv_key = source["awg_client_priv_key"];
	        this.awg_client_pub_key = source["awg_client_pub_key"];
	        this.awg_server_pub_key = source["awg_server_pub_key"];
	        this.awg_client_config = source["awg_client_config"];
	        this.awg_exe_path = source["awg_exe_path"];
	        this.awg_interface = source["awg_interface"];
	        this.active_decoy_domain = source["active_decoy_domain"];
	        this.decoy_domains = source["decoy_domains"];
	        this.hotswap_enabled = source["hotswap_enabled"];
	        this.hotswap_interval_min = source["hotswap_interval_min"];
	        this.lmstudio_model = source["lmstudio_model"];
	        this.ai_system_prompt = source["ai_system_prompt"];
	        this.cover_sites = source["cover_sites"];
	        this.behavior_profile = source["behavior_profile"];
	        this.cover_lists = this.convertValues(source["cover_lists"], CoverList);
	        this.active_cover_list_id = source["active_cover_list_id"];
	        this.intensity = source["intensity"];
	        this.session_minutes = source["session_minutes"];
	        this.memory_timeout_policy = source["memory_timeout_policy"];
	        this.adaptive_control = source["adaptive_control"];
	        this.adaptive_mode = source["adaptive_mode"];
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
	    id: string;
	    name: string;
	    vps_host: string;
	    clients: number;
	    active: boolean;
	    deployed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ServerSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.vps_host = source["vps_host"];
	        this.clients = source["clients"];
	        this.active = source["active"];
	        this.deployed = source["deployed"];
	    }
	}

}

