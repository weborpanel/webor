/** Panel JSON API client: same-origin credentials, 401 redirect, uniform errors. */
import { goto } from '$app/navigation';

export type ApiBase = {
	ok?: boolean;
	error?: string;
	message?: string;
	hint?: string;
	approval_required?: boolean;
	approval_id?: string;
	approval_operation?: string;
};

export class ApiError extends Error {
	status: number;
	data: ApiBase;

	constructor(message: string, status = 0, data: ApiBase = {}) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.data = data;
	}
}

export const NETWORK_ERROR = 'Could not reach the panel API';

export type RequestOptions = {
	method?: string;
	/** JSON body (sets Content-Type). */
	json?: unknown;
	/** Raw body (FormData, ArrayBuffer, …). */
	body?: BodyInit;
	headers?: Record<string, string>;
	signal?: AbortSignal;
};

export type ApiResponse<T> = { status: number; ok: boolean; data: T & ApiBase };

/**
 * Performs a request and always resolves with the parsed body, even for
 * non-2xx responses (callers inspect `approval_required` etc.). Throws
 * ApiError only on network failure or 401 (after redirecting to /login).
 */
export async function request<T = Record<string, unknown>>(
	path: string,
	opts: RequestOptions = {}
): Promise<ApiResponse<T>> {
	const headers: Record<string, string> = { ...opts.headers };
	let body = opts.body;
	if (opts.json !== undefined) {
		headers['Content-Type'] = 'application/json';
		body = JSON.stringify(opts.json);
	}
	let res: Response;
	try {
		res = await fetch(path, {
			method: opts.method ?? (body ? 'POST' : 'GET'),
			credentials: 'same-origin',
			headers,
			body,
			signal: opts.signal
		});
	} catch (e) {
		if (e instanceof DOMException && e.name === 'AbortError') throw e;
		throw new ApiError(NETWORK_ERROR);
	}
	if (res.status === 401) {
		await goto('/login');
		throw new ApiError('Session expired', 401);
	}
	const data = (await res.json().catch(() => ({}))) as T & ApiBase;
	return { status: res.status, ok: res.ok && !!data.ok, data };
}

/** Like request() but throws ApiError unless the response is ok. */
export async function api<T = Record<string, unknown>>(
	path: string,
	opts: RequestOptions = {}
): Promise<T & ApiBase> {
	const r = await request<T>(path, opts);
	if (!r.ok) {
		throw new ApiError(
			r.data.error || r.data.message || `Request failed (${r.status})`,
			r.status,
			r.data
		);
	}
	return r.data;
}

export function errorMessage(e: unknown, fallback = NETWORK_ERROR): string {
	if (e instanceof Error && e.message) return e.message;
	return fallback;
}

export const sitePath = (name: string, rest = '') =>
	`/api/sites/${encodeURIComponent(name)}${rest}`;
