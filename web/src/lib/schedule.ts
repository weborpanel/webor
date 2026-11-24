/** Shared cron schedule helpers for backups and site crons (5-field cron). */

export type SchedulePreset = 'hourly' | 'daily' | 'weekdays' | 'weekly' | 'custom';

export type MatchedPreset = {
	preset: SchedulePreset;
	hour: number;
	minute: number;
	weekday: number; // 0=Sun … 6=Sat
};

const DOW_NAMES = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'] as const;

const HOURLY = '0 * * * *';

export function isValidCron(expr: string): boolean {
	const fields = expr.trim().split(/\s+/).filter(Boolean);
	return fields.length === 5;
}

function pad2(n: number): string {
	return String(n).padStart(2, '0');
}

function formatTime(hour: number, minute: number): string {
	const h12 = hour % 12 || 12;
	const ampm = hour < 12 ? 'AM' : 'PM';
	return `${h12}:${pad2(minute)} ${ampm}`;
}

function parseIntField(field: string): number | null {
	if (!/^\d+$/.test(field)) return null;
	return Number(field);
}

export function cronFromPreset(
	preset: Exclude<SchedulePreset, 'custom'>,
	opts: { hour?: number; minute?: number; weekday?: number } = {}
): string {
	const hour = clamp(opts.hour ?? 3, 0, 23);
	const minute = clamp(opts.minute ?? 0, 0, 59);
	const weekday = clamp(opts.weekday ?? 0, 0, 6);
	switch (preset) {
		case 'hourly':
			return HOURLY;
		case 'daily':
			return `${minute} ${hour} * * *`;
		case 'weekdays':
			return `${minute} ${hour} * * 1-5`;
		case 'weekly':
			return `${minute} ${hour} * * ${weekday}`;
	}
}

function clamp(n: number, min: number, max: number): number {
	if (!Number.isFinite(n)) return min;
	return Math.min(max, Math.max(min, Math.trunc(n)));
}

/** Match a cron expression to a known preset, or custom. */
export function matchPreset(expr: string): MatchedPreset {
	const raw = expr.trim();
	if (!raw) {
		return { preset: 'custom', hour: 3, minute: 0, weekday: 0 };
	}
	if (raw === HOURLY) {
		return { preset: 'hourly', hour: 0, minute: 0, weekday: 0 };
	}

	const fields = raw.split(/\s+/);
	if (fields.length !== 5) {
		return { preset: 'custom', hour: 3, minute: 0, weekday: 0 };
	}

	const [minF, hourF, dom, month, dow] = fields;
	const minute = parseIntField(minF);
	const hour = parseIntField(hourF);
	if (minute === null || hour === null || minute > 59 || hour > 23) {
		return { preset: 'custom', hour: 3, minute: 0, weekday: 0 };
	}
	if (dom !== '*' || month !== '*') {
		return { preset: 'custom', hour, minute, weekday: 0 };
	}

	if (dow === '*') {
		return { preset: 'daily', hour, minute, weekday: 0 };
	}
	if (dow === '1-5') {
		return { preset: 'weekdays', hour, minute, weekday: 1 };
	}
	const weekday = parseIntField(dow);
	if (weekday !== null && weekday >= 0 && weekday <= 6) {
		return { preset: 'weekly', hour, minute, weekday };
	}

	return { preset: 'custom', hour, minute, weekday: 0 };
}

/** Human-readable summary; falls back to the raw expression for unknown patterns. */
export function describeCron(expr: string | undefined | null): string {
	const raw = (expr ?? '').trim();
	if (!raw) return 'No schedule';

	if (raw === HOURLY || raw === '0 * * * *') {
		return 'Every hour';
	}

	// Every N minutes: */N * * * *
	const everyMin = raw.match(/^\*\/(\d+) \* \* \* \*$/);
	if (everyMin) {
		const n = Number(everyMin[1]);
		return n === 1 ? 'Every minute' : `Every ${n} minutes`;
	}

	const matched = matchPreset(raw);
	if (matched.preset === 'daily') {
		return `Daily at ${formatTime(matched.hour, matched.minute)}`;
	}
	if (matched.preset === 'weekdays') {
		return `Weekdays at ${formatTime(matched.hour, matched.minute)}`;
	}
	if (matched.preset === 'weekly') {
		return `${DOW_NAMES[matched.weekday]}s at ${formatTime(matched.hour, matched.minute)}`;
	}
	if (matched.preset === 'hourly') {
		return 'Every hour';
	}

	return raw;
}

export const PRESET_OPTIONS: { value: SchedulePreset; label: string }[] = [
	{ value: 'hourly', label: 'Every hour' },
	{ value: 'daily', label: 'Daily' },
	{ value: 'weekdays', label: 'Weekdays' },
	{ value: 'weekly', label: 'Weekly' },
	{ value: 'custom', label: 'Custom' }
];

export const WEEKDAY_OPTIONS = DOW_NAMES.map((label, value) => ({ value, label }));
