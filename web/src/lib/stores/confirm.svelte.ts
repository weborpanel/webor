export type ConfirmOptions = {
	title: string;
	message?: string;
	/** Bullet points rendered under the message. */
	details?: string[];
	confirmLabel?: string;
	cancelLabel?: string;
	danger?: boolean;
	/** When set, the user must type this exact text to enable confirm. */
	typeToConfirm?: string;
};

type Pending = ConfirmOptions & { resolve: (ok: boolean) => void };

class ConfirmStore {
	current = $state<Pending | null>(null);

	ask(opts: ConfirmOptions): Promise<boolean> {
		this.current?.resolve(false);
		return new Promise((resolve) => {
			this.current = { ...opts, resolve };
		});
	}

	settle(ok: boolean) {
		const c = this.current;
		this.current = null;
		c?.resolve(ok);
	}
}

export const confirmer = new ConfirmStore();

export const confirmDialog = (opts: ConfirmOptions) => confirmer.ask(opts);
