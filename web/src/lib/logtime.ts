/** Display helpers: show log timestamps in the panel-configured IANA zone. */

const ISO_RE =
	/\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:?\d{2})?/g;

/** Caddy JSON access log unix seconds (optional fraction). */
const CADDY_TS_RE = /("ts"\s*:\s*)(\d+(?:\.\d+)?)/g;

/** Normalize offsets like +0000 / +0530 to +00:00 / +05:30 for Date parsing. */
function normalizeIso(raw: string): string {
	let s = raw.trim();
	if (/[+-]\d{4}$/.test(s)) {
		s = s.replace(/([+-]\d{2})(\d{2})$/, '$1:$2');
	}
	if (s.includes(' ') && !s.includes('T')) {
		s = s.replace(' ', 'T');
	}
	return s;
}

export function formatInTimeZone(
	input: string | number | Date,
	timeZone: string,
	opts: Intl.DateTimeFormatOptions = {
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		second: '2-digit',
		hourCycle: 'h23'
	}
): string {
	const d =
		input instanceof Date
			? input
			: typeof input === 'number'
				? new Date(input)
				: new Date(normalizeIso(input));
	if (Number.isNaN(d.getTime())) return typeof input === 'string' ? input : '';
	try {
		return new Intl.DateTimeFormat('en-CA', { ...opts, timeZone }).format(d);
	} catch {
		return d.toISOString();
	}
}

/** RFC3339-like stamp in the target zone (with numeric offset). */
export function formatStampInTimeZone(input: string | number | Date, timeZone: string): string {
	const d =
		input instanceof Date
			? input
			: typeof input === 'number'
				? new Date(input)
				: new Date(normalizeIso(input));
	if (Number.isNaN(d.getTime())) return typeof input === 'string' ? input : '';
	try {
		const parts = new Intl.DateTimeFormat('en-US', {
			timeZone,
			year: 'numeric',
			month: '2-digit',
			day: '2-digit',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
			hourCycle: 'h23',
			timeZoneName: 'longOffset'
		}).formatToParts(d);
		const get = (type: Intl.DateTimeFormatPartTypes) =>
			parts.find((p) => p.type === type)?.value ?? '';
		const offsetRaw = get('timeZoneName'); // GMT+5:30 or UTC
		let offset = 'Z';
		if (offsetRaw.startsWith('GMT')) {
			const rest = offsetRaw.slice(3);
			if (!rest || rest === '+0' || rest === '+00:00') offset = 'Z';
			else if (/^[+-]\d{1,2}$/.test(rest)) {
				const sign = rest[0];
				const h = rest.slice(1).padStart(2, '0');
				offset = `${sign}${h}:00`;
			} else if (/^[+-]\d{1,2}:\d{2}$/.test(rest)) {
				const sign = rest[0];
				const [h, m] = rest.slice(1).split(':');
				offset = `${sign}${h.padStart(2, '0')}:${m}`;
			} else offset = rest;
		} else if (offsetRaw === 'UTC') {
			offset = 'Z';
		}
		return `${get('year')}-${get('month')}-${get('day')}T${get('hour')}:${get('minute')}:${get('second')}${offset}`;
	} catch {
		return d.toISOString();
	}
}

function remapIsoMatch(match: string, timeZone: string): string {
	const d = new Date(normalizeIso(match));
	if (Number.isNaN(d.getTime())) return match;
	return formatStampInTimeZone(d, timeZone);
}

/**
 * Rewrite recognizable timestamps in log text for display.
 * Does not mutate on-disk files — call only for UI rendering.
 */
export function remapLogText(text: string, timeZone?: string): string {
	const tz = (timeZone || '').trim();
	if (!tz || !text) return text;
	let out = text.replace(ISO_RE, (m) => remapIsoMatch(m, tz));
	out = out.replace(CADDY_TS_RE, (_full, prefix: string, sec: string) => {
		const ms = Number(sec) * 1000;
		if (!Number.isFinite(ms)) return _full;
		const stamp = formatStampInTimeZone(ms, tz);
		// Keep a readable annotation beside the original unix field.
		return `${prefix}${sec} /* ${stamp} */`;
	});
	return out;
}
