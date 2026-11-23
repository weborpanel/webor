/** Shared panel auth helpers (client-side). */

export type AuthStatus = {
	ok: boolean;
	authenticated: boolean;
	needs_setup: boolean;
};

/** Where the panel lands after login or setup. */
export const HOME_PATH = '/dashboard';

export async function fetchAuthStatus(): Promise<AuthStatus> {
	const res = await fetch('/api/auth/status', { credentials: 'same-origin' });
	const data = (await res.json().catch(() => ({}))) as Partial<AuthStatus>;
	if (!res.ok || !data.ok) {
		return { ok: false, authenticated: false, needs_setup: false };
	}
	return {
		ok: true,
		authenticated: !!data.authenticated,
		needs_setup: !!data.needs_setup
	};
}

/** Routes that do not require a session. */
export function isPublicPath(pathname: string): boolean {
	return pathname === '/' || pathname.startsWith('/login') || pathname.startsWith('/setup');
}

const PROTECTED_PREFIXES = ['/dashboard', '/sites', '/activity', '/account', '/backups'];

/** Panel routes that require an authenticated session. */
export function isProtectedPath(pathname: string): boolean {
	return PROTECTED_PREFIXES.some((p) => pathname === p || pathname.startsWith(`${p}/`));
}
