import type { OverviewSite } from '#lib/types.ts';

export type StatusTone = 'ok' | 'warn' | 'danger' | 'neutral' | 'info';

/** Human status for a site row. Apps are socket-activated, so "not running" usually means idle. */
export function siteStatus(site: Pick<OverviewSite, 'type' | 'sampled' | 'active'>): {
	tone: StatusTone;
	label: string;
} {
	if (site.type !== 'app') return { tone: 'ok', label: 'Serving' };
	if (!site.sampled) return { tone: 'neutral', label: 'Pending' };
	return site.active ? { tone: 'ok', label: 'Running' } : { tone: 'neutral', label: 'Idle' };
}
