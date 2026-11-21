/** Shapes returned by the panel API. */

export type Series = {
	range: string;
	step: number;
	from: number;
	to: number;
	t: number[];
	series: Record<string, number[]>;
};

export type HostSnapshot = {
	hostname: string;
	kernel: string;
	cpu_count: number;
	uptime_sec: number;
	cpu_pct: number;
	mem_used: number;
	mem_total: number;
	load1: number;
	load5: number;
	load15: number;
	disk_used: number;
	disk_total: number;
	net_rx: number;
	net_tx: number;
};

export type OverviewSite = {
	name: string;
	type: string;
	parent?: string;
	https: boolean;
	socket_proxy?: boolean;
	database_count: number;
	cache: boolean;
	subdomain_count?: number;
	sampled: boolean;
	active: boolean;
	cpu_pct: number;
	mem_bytes: number;
	disk_bytes: number;
	memory_max?: string;
	mem_limit_bytes?: number;
	disk_quota?: string;
	disk_quota_bytes?: number;
	spark?: Series | null;
};

export type Overview = {
	sampled: boolean;
	host: HostSnapshot;
	host_spark?: Series | null;
	sites: OverviewSite[];
	pending_approvals: number;
	version: string;
	https_disabled?: boolean;
};

export type SiteConfig = {
	name: string;
	type?: string;
	https?: boolean;
	root?: string;
	socket_proxy?: boolean;
	cron?: string[];
	jobs?: string[];
	parent?: string;
	cache?: string;
	databases?: { id?: string; engine?: string }[];
	subdomain_count?: number;
};

export type MetricsRange = '1h' | '24h' | '7d';

export type AuditEvent = {
	id: number;
	ts: string;
	actor: string;
	operation: string;
	args: Record<string, unknown>;
	result: string;
};
