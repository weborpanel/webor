import { request, errorMessage, type ApiBase } from '#lib/api.ts';

export type ApprovalStatus = '' | 'pending' | 'approved' | 'used' | 'expired';

type FlowOptions<A> = {
	/** Helper operation name for an action (e.g. EnsureSiteDatabase). */
	operationFor: (action: A) => string;
	/** Args bound into the approval request. */
	argsFor: (operation: string, action: A | null) => Record<string, unknown>;
	/** Re-run the action once the approval is granted. */
	resume: (action: A, approvalId: string) => Promise<void>;
	onError?: (message: string) => void;
};

/**
 * Step-up approval state machine shared by database, cache and site
 * deletion: request an approval id, show `sudo wbr approve <id>`, poll
 * until it is approved, then resume the original action once.
 */
export class ApprovalFlow<A> {
	pending = $state<A | null>(null);
	required = $state(false);
	id = $state('');
	hint = $state('');
	operation = $state('');
	status = $state<ApprovalStatus>('');
	autoBusy = $state(false);
	busy = $state(false);

	#triedFor = '';
	#opts: FlowOptions<A>;

	constructor(opts: FlowOptions<A>) {
		this.#opts = opts;
	}

	get active() {
		return this.required || !!this.hint;
	}

	same(a: A | null, b: A): boolean {
		return !!a && JSON.stringify(a) === JSON.stringify(b);
	}

	/** Clears a stale approval when the user starts a different action. */
	resetIfDifferent(action: A) {
		if (this.id.trim() && !this.same(this.pending, action)) this.clear();
	}

	async begin(data: ApiBase, action: A) {
		this.pending = action;
		this.required = true;
		this.status = '';
		this.#triedFor = '';
		this.operation = data.approval_operation || this.#opts.operationFor(action);
		if (data.approval_id) {
			this.id = data.approval_id;
			this.hint = data.hint || `sudo wbr approve ${data.approval_id}`;
		} else {
			this.hint = data.hint || data.message || data.error || 'Root approval required';
		}
		if (!this.id.trim()) await this.requestApproval();
	}

	async requestApproval() {
		const operation =
			this.operation.trim() || (this.pending ? this.#opts.operationFor(this.pending) : '');
		if (!operation) {
			this.#opts.onError?.('No approval operation available; retry the action first');
			return;
		}
		this.busy = true;
		try {
			const r = await request<{ approval_id?: string }>('/api/approvals', {
				method: 'POST',
				json: { operation, args: this.#opts.argsFor(operation, this.pending) }
			});
			if (!r.ok) {
				this.#opts.onError?.(r.data.error || 'Could not request approval');
				return;
			}
			this.operation = operation;
			this.id = r.data.approval_id || '';
			this.status = 'pending';
			this.hint = this.id ? `Run on the server:\nsudo wbr approve ${this.id}` : r.data.hint || 'Approval created';
		} catch (e) {
			this.#opts.onError?.(errorMessage(e));
		} finally {
			this.busy = false;
		}
	}

	async poll() {
		const id = this.id.trim();
		if (!id || this.busy || this.autoBusy) return;
		let data: ApiBase & { status?: ApprovalStatus; used?: boolean; operation?: string; args_json?: string };
		try {
			const r = await request<{ status?: ApprovalStatus; used?: boolean; operation?: string; args_json?: string }>(
				`/api/approvals/${encodeURIComponent(id)}`
			);
			if (!r.ok) return;
			data = r.data;
		} catch {
			return;
		}
		this.status = data.status || '';
		if (data.operation) this.operation = data.operation;
		if (data.status === 'approved' && !data.used) {
			if (this.#triedFor === id || !this.pending) return;
			this.#triedFor = id;
			this.hint = 'Approved. Continuing…';
			this.autoBusy = true;
			try {
				await this.#opts.resume(this.pending, id);
			} finally {
				this.autoBusy = false;
			}
		} else if (data.status === 'pending') {
			this.hint = `Run on the server:\nsudo wbr approve ${id}`;
		} else if (data.status === 'expired') {
			this.hint = 'Approval expired. Retry the action.';
			this.id = '';
		} else if (data.status === 'used') {
			this.hint = 'Approval already used. Retry the action.';
			this.id = '';
		}
	}

	clear() {
		this.pending = null;
		this.required = false;
		this.id = '';
		this.hint = '';
		this.operation = '';
		this.status = '';
		this.autoBusy = false;
		this.#triedFor = '';
	}
}
