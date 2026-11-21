import { byteTicks, formatBytes, formatPct, formatRate, niceTicks } from '#lib/format.ts';

export type ValueKind = 'pct' | 'bytes' | 'rate' | 'number';

export function formatValue(kind: ValueKind, v: number): string {
	switch (kind) {
		case 'pct':
			return formatPct(v);
		case 'bytes':
			return formatBytes(v);
		case 'rate':
			return formatRate(v);
		default:
			return v >= 100 ? Math.round(v).toString() : v.toFixed(2);
	}
}

/** Compact axis label (no decimals where avoidable). */
export function formatTick(kind: ValueKind, v: number): string {
	switch (kind) {
		case 'pct':
			return `${Math.round(v)}%`;
		case 'bytes':
			return formatBytes(v, 0);
		case 'rate':
			return `${formatBytes(v, 0)}/s`;
		default:
			return Number.isInteger(v) ? v.toString() : v.toFixed(1);
	}
}

export function yTicks(kind: ValueKind, max: number): number[] {
	if (kind === 'bytes' || kind === 'rate') return byteTicks(max);
	if (kind === 'pct') return niceTicks(Math.min(Math.max(max, 5), 100));
	return niceTicks(max);
}

export type Point = { x: number; y: number };

/**
 * Splits points into contiguous runs, breaking where consecutive samples are
 * further apart than `maxGap` seconds (sampler down, panel restarted).
 */
export function segments(t: number[], values: number[], maxGap: number): number[][] {
	const out: number[][] = [];
	let cur: number[] = [];
	for (let i = 0; i < t.length; i++) {
		const v = values[i];
		if (v == null || Number.isNaN(v)) {
			if (cur.length) out.push(cur);
			cur = [];
			continue;
		}
		if (cur.length && t[i] - t[cur[cur.length - 1]] > maxGap) {
			out.push(cur);
			cur = [];
		}
		cur.push(i);
	}
	if (cur.length) out.push(cur);
	return out;
}

export function linePath(points: Point[]): string {
	if (!points.length) return '';
	if (points.length === 1) {
		const p = points[0];
		return `M${p.x - 1.5},${p.y}L${p.x + 1.5},${p.y}`;
	}
	return points.map((p, i) => `${i ? 'L' : 'M'}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join('');
}

export function areaPath(points: Point[], baseline: number): string {
	if (points.length < 2) return '';
	const first = points[0];
	const last = points[points.length - 1];
	return `${linePath(points)}L${last.x.toFixed(1)},${baseline}L${first.x.toFixed(1)},${baseline}Z`;
}

/** Index of the sample nearest to time `x`. */
export function nearestIndex(t: number[], x: number): number {
	if (!t.length) return -1;
	let lo = 0;
	let hi = t.length - 1;
	while (lo < hi) {
		const mid = (lo + hi) >> 1;
		if (t[mid] < x) lo = mid + 1;
		else hi = mid;
	}
	if (lo > 0 && Math.abs(t[lo - 1] - x) < Math.abs(t[lo] - x)) return lo - 1;
	return lo;
}
