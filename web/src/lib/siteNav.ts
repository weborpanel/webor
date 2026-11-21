import type { TabItem } from '#lib/components/Tabs.svelte';

export type SiteSection = TabItem & { key: string };

/** Per-site sub-navigation shared by the sidebar, site tabs and command palette. */
export function siteSections(name: string, type?: string): SiteSection[] {
	const base = `/sites/${encodeURIComponent(name)}`;
	const isApp = type === 'app';
	const sections: SiteSection[] = [
		{ key: 'overview', href: base, label: 'Overview', icon: 'dashboard', exact: true },
		{ key: 'metrics', href: `${base}/metrics`, label: 'Metrics', icon: 'chart' }
	];
	if (isApp) {
		sections.push({ key: 'deploy', href: `${base}/deploy`, label: 'Deploy', icon: 'rocket' });
	}
	sections.push(
		{ key: 'files', href: `${base}/files`, label: 'Files', icon: 'file' },
		{ key: 'databases', href: `${base}/databases`, label: 'Databases', icon: 'database' },
		{ key: 'cache', href: `${base}/cache`, label: 'Cache', icon: 'zap' },
		{ key: 'domains', href: `${base}/domains`, label: 'Domains & SSL', icon: 'globe' },
		{ key: 'logs', href: `${base}/logs`, label: 'Logs', icon: 'terminal' },
		{ key: 'settings', href: `${base}/settings`, label: 'Settings', icon: 'settings' }
	);
	return sections;
}

/** Decodes the site name from a /sites/<name>/… path, or '' when not on a site. */
export function siteFromPath(pathname: string): string {
	const m = /^\/sites\/([^/]+)/.exec(pathname);
	if (!m) return '';
	try {
		return decodeURIComponent(m[1]);
	} catch {
		return m[1];
	}
}
