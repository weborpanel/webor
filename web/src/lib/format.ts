/** Display formatters shared by dashboard, charts and site pages. */

const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];

export function formatBytes(n?: number | null, digits = 1): string {
	if (n == null || n < 0 || Number.isNaN(n)) return '—';
	let v = n;
	let i = 0;
	while (v >= 1024 && i < BYTE_UNITS.length - 1) {
		v /= 1024;
		i++;
	}
	const fixed = v < 10 && i > 0 ? v.toFixed(digits) : Math.round(v).toString();
	return `${fixed} ${BYTE_UNITS[i]}`;
}

export function formatRate(bytesPerSec?: number | null): string {
	if (bytesPerSec == null || Number.isNaN(bytesPerSec)) return '—';
	return `${formatBytes(bytesPerSec)}/s`;
}

export function formatPct(v?: number | null, digits = 1): string {
	if (v == null || Number.isNaN(v)) return '—';
	if (v > 0 && v < 0.1) return '<0.1%';
	return `${v.toFixed(v >= 10 ? 0 : digits)}%`;
}

export function formatDuration(totalSec?: number | null): string {
	if (totalSec == null || totalSec < 0) return '—';
	const d = Math.floor(totalSec / 86400);
	const h = Math.floor((totalSec % 86400) / 3600);
	const m = Math.floor((totalSec % 3600) / 60);
	if (d > 0) return `${d}d ${h}h`;
	if (h > 0) return `${h}h ${m}m`;
	return `${m}m`;
}

export function relativeTime(input: string | number | Date): string {
	const t = input instanceof Date ? input.getTime() : typeof input === 'number' ? input : Date.parse(input);
	if (Number.isNaN(t)) return '';
	const diff = (Date.now() - t) / 1000;
	const abs = Math.abs(diff);
	const fmt = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });
	const sign = diff >= 0 ? -1 : 1;
	if (abs < 45) return fmt.format(sign * Math.round(abs), 'second');
	if (abs < 2700) return fmt.format(sign * Math.round(abs / 60), 'minute');
	if (abs < 86400) return fmt.format(sign * Math.round(abs / 3600), 'hour');
	if (abs < 86400 * 30) return fmt.format(sign * Math.round(abs / 86400), 'day');
	return new Date(t).toLocaleDateString();
}

/** Axis label for a unix-seconds timestamp, coarser for long windows. */
export function formatAxisTime(sec: number, spanSec: number): string {
	const d = new Date(sec * 1000);
	if (spanSec > 2 * 86400) {
		return d.toLocaleDateString(undefined, { weekday: 'short', day: 'numeric' });
	}
	return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
}

export function formatTooltipTime(sec: number): string {
	return new Date(sec * 1000).toLocaleString(undefined, {
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit'
	});
}

/** "Nice" tick values covering [0, max]. */
export function niceTicks(max: number, count = 4): number[] {
	if (!(max > 0)) return [0];
	const raw = max / count;
	const mag = 10 ** Math.floor(Math.log10(raw));
	const norm = raw / mag;
	const step = (norm <= 1 ? 1 : norm <= 2 ? 2 : norm <= 2.5 ? 2.5 : norm <= 5 ? 5 : 10) * mag;
	const ticks: number[] = [];
	for (let v = 0; v <= max + step * 0.001; v += step) ticks.push(v);
	if (ticks[ticks.length - 1] < max) ticks.push(ticks[ticks.length - 1] + step);
	return ticks;
}

/** Byte-aware ticks (powers of 1024 multiples) covering [0, max]. */
export function byteTicks(max: number, count = 4): number[] {
	if (!(max > 0)) return [0];
	let unit = 1;
	while (max / unit >= 1024) unit *= 1024;
	return niceTicks(max / unit, count).map((v) => v * unit);
}

export function pctOf(used?: number | null, total?: number | null): number {
	if (!used || !total || total <= 0) return 0;
	return Math.min(100, Math.max(0, (used / total) * 100));
}

/** Severity bucket for a utilisation percentage. */
export function usageLevel(pct: number): 'ok' | 'warn' | 'danger' {
	if (pct >= 90) return 'danger';
	if (pct >= 75) return 'warn';
	return 'ok';
}
