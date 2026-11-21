import type { AuditEvent } from '#lib/types.ts';
import type { IconName } from '#lib/icons/paths.ts';

/** "EnsureSiteDatabase" → "Ensure site database". */
export function humanizeOperation(op: string): string {
	const words = op.replace(/([a-z0-9])([A-Z])/g, '$1 $2').replace(/([A-Z]+)([A-Z][a-z])/g, '$1 $2');
	return words.charAt(0).toUpperCase() + words.slice(1).toLowerCase();
}

export function eventSite(e: AuditEvent): string {
	const a = e.args ?? {};
	for (const k of ['domain', 'site', 'name', 'child']) {
		const v = a[k];
		if (typeof v === 'string' && v) return v;
	}
	return '';
}

export function eventFailed(e: AuditEvent): boolean {
	const r = (e.result || '').toLowerCase();
	return !!r && r !== 'ok' && (r.startsWith('error') || r.includes('fail') || r.includes('denied'));
}

export function eventIcon(op: string): IconName {
	const o = op.toLowerCase();
	if (o.includes('database') || o.includes('postgres') || o.includes('maria')) return 'database';
	if (o.includes('valkey') || o.includes('cache')) return 'zap';
	if (o.includes('deploy') || o.includes('release') || o.includes('build')) return 'rocket';
	if (o.includes('delete') || o.includes('drop') || o.includes('remove')) return 'trash';
	if (o.includes('caddy') || o.includes('https') || o.includes('cert')) return 'lock';
	if (o.includes('approv')) return 'shield';
	if (o.includes('file') || o.includes('upload')) return 'upload';
	if (o.includes('user') || o.includes('site')) return 'globe';
	return 'activity';
}
