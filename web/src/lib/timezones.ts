/** IANA zones for the Account schedule-timezone picker. */

const EXTRA = [
	'UTC',
	'Asia/Kolkata',
	'Asia/Calcutta',
	'America/New_York',
	'America/Chicago',
	'America/Denver',
	'America/Los_Angeles',
	'America/Sao_Paulo',
	'Europe/London',
	'Europe/Berlin',
	'Europe/Paris',
	'Europe/Moscow',
	'Africa/Cairo',
	'Africa/Johannesburg',
	'Asia/Dubai',
	'Asia/Singapore',
	'Asia/Tokyo',
	'Asia/Shanghai',
	'Australia/Sydney',
	'Pacific/Auckland'
] as const;

let cached: string[] | null = null;

/** Full IANA list from the runtime, plus a few common aliases browsers may omit. */
export function listTimeZones(): string[] {
	if (cached) return cached;
	const set = new Set<string>();
	try {
		if (typeof Intl !== 'undefined' && 'supportedValuesOf' in Intl) {
			for (const z of Intl.supportedValuesOf('timeZone')) set.add(z);
		}
	} catch {
		/* ignore */
	}
	for (const z of EXTRA) set.add(z);
	cached = [...set].sort((a, b) => a.localeCompare(b));
	return cached;
}
