import { api, errorMessage } from '#lib/api.ts';
import type { MetricsRange, Series } from '#lib/types.ts';

type MetricsResponse = {
	metrics: Series;
	mem_limit_bytes?: number;
	disk_quota_bytes?: number;
};

/** Loads a metrics time series for a range; components drive refresh. */
export class MetricsQuery {
	range = $state<MetricsRange>('1h');
	data = $state.raw<Series | null>(null);
	memLimit = $state(0);
	diskQuota = $state(0);
	loading = $state(false);
	error = $state('');
	#path: string;
	#seq = 0;

	constructor(path: string, range: MetricsRange = '1h') {
		this.#path = path;
		this.range = range;
	}

	async load() {
		const seq = ++this.#seq;
		this.loading = true;
		try {
			const r = await api<MetricsResponse>(`${this.#path}?range=${this.range}`);
			if (seq !== this.#seq) return;
			this.data = r.metrics;
			this.memLimit = r.mem_limit_bytes ?? 0;
			this.diskQuota = r.disk_quota_bytes ?? 0;
			this.error = '';
		} catch (e) {
			if (seq === this.#seq) this.error = errorMessage(e, 'Could not load metrics');
		} finally {
			if (seq === this.#seq) this.loading = false;
		}
	}

	setRange(r: MetricsRange) {
		this.range = r;
		return this.load();
	}

	values(key: string): number[] {
		return this.data?.series[key] ?? [];
	}
}

/** Runs `fn` every `ms` while the tab is visible. Returns a cleanup. */
export function poll(fn: () => void, ms: number): () => void {
	const timer = setInterval(() => {
		if (document.visibilityState === 'visible') fn();
	}, ms);
	return () => clearInterval(timer);
}
