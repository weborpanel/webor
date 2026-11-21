import { createContext } from 'svelte';
import { api, errorMessage } from '#lib/api.ts';
import type { SiteConfig } from '#lib/types.ts';

/** Config for the site being viewed, loaded once by sites/[name]/+layout and shared with its tabs. */
export class SiteState {
	name = $state('');
	site = $state.raw<SiteConfig | null>(null);
	loading = $state(true);
	error = $state('');
	labDisabled = $state(false);

	type = $derived(this.site?.type || 'static');
	isApp = $derived(this.type === 'app');
	parent = $derived(this.site?.parent || '');
	isChild = $derived(!!this.parent);
	root = $derived(this.site?.root || 'public');
	https = $derived(!!this.site?.https);
	socketProxy = $derived(!!this.site?.socket_proxy);
	cron = $derived(this.site?.cron ?? []);
	jobs = $derived(this.site?.jobs ?? []);
	exists = $derived(!!this.site);

	async load(name = this.name) {
		if (name !== this.name) this.site = null;
		this.name = name;
		this.loading = true;
		this.error = '';
		try {
			const data = await api<{ sites?: SiteConfig[]; lab_disabled?: boolean }>('/api/sites');
			if (name !== this.name) return;
			this.site = (data.sites || []).find((s) => s.name.toLowerCase() === name.toLowerCase()) ?? null;
			if (data.lab_disabled) this.labDisabled = true;
		} catch (e) {
			this.error = errorMessage(e, 'Failed to load site');
		} finally {
			if (name === this.name) this.loading = false;
		}
	}

	setHttps(on: boolean) {
		if (this.site) this.site = { ...this.site, https: on };
	}

	/** API path for this site, e.g. path('/logs') → /api/sites/example.com/logs */
	path(rest = '') {
		return `/api/sites/${encodeURIComponent(this.name)}${rest}`;
	}

	/** Panel URL for this site, e.g. href('/logs') → /sites/example.com/logs */
	href(rest = '') {
		return `/sites/${encodeURIComponent(this.name)}${rest}`;
	}
}

export const [getSite, setSite] = createContext<SiteState>();
