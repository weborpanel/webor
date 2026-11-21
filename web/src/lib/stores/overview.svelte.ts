import { api, errorMessage } from '#lib/api.ts';
import type { Overview } from '#lib/types.ts';

/** Cached /api/overview: host snapshot + site summaries shared by the shell, dashboard and site list. */
class OverviewStore {
	data = $state.raw<Overview | null>(null);
	loading = $state(false);
	error = $state('');
	loadedAt = $state(0);
	#inflight: Promise<void> | null = null;

	get sites() {
		return this.data?.sites ?? [];
	}

	refresh(): Promise<void> {
		if (this.#inflight) return this.#inflight;
		this.loading = true;
		this.#inflight = (async () => {
			try {
				this.data = await api<Overview>('/api/overview');
				this.error = '';
				this.loadedAt = Date.now();
			} catch (e) {
				this.error = errorMessage(e, 'Could not load overview');
			} finally {
				this.loading = false;
				this.#inflight = null;
			}
		})();
		return this.#inflight;
	}

	/** Refresh only when older than maxAgeMs. */
	ensure(maxAgeMs = 15_000) {
		if (!this.data || Date.now() - this.loadedAt > maxAgeMs) return this.refresh();
		return Promise.resolve();
	}

	clear() {
		this.data = null;
		this.loadedAt = 0;
	}
}

export const overview = new OverviewStore();

const RECENT_KEY = 'wbr-recent-sites';

class RecentSites {
	names = $state<string[]>(read());

	visit(name: string) {
		const n = name.toLowerCase();
		this.names = [n, ...this.names.filter((x) => x !== n)].slice(0, 5);
		try {
			localStorage.setItem(RECENT_KEY, JSON.stringify(this.names));
		} catch {
			// storage blocked
		}
	}

	forget(name: string) {
		this.names = this.names.filter((x) => x !== name.toLowerCase());
		try {
			localStorage.setItem(RECENT_KEY, JSON.stringify(this.names));
		} catch {
			// storage blocked
		}
	}
}

function read(): string[] {
	try {
		const v = JSON.parse(localStorage.getItem(RECENT_KEY) || '[]');
		return Array.isArray(v) ? v.filter((x) => typeof x === 'string').slice(0, 5) : [];
	} catch {
		return [];
	}
}

export const recentSites = new RecentSites();
