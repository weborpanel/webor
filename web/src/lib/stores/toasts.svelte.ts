export type ToastKind = 'success' | 'error' | 'info' | 'warn';

export type Toast = {
	id: number;
	kind: ToastKind;
	message: string;
};

class ToastStore {
	items = $state<Toast[]>([]);
	#next = 1;

	push(kind: ToastKind, message: string, timeoutMs = kind === 'error' ? 7000 : 4000) {
		const id = this.#next++;
		this.items.push({ id, kind, message });
		if (timeoutMs > 0) setTimeout(() => this.dismiss(id), timeoutMs);
		return id;
	}

	success(message: string) {
		return this.push('success', message);
	}

	error(message: string) {
		return this.push('error', message);
	}

	info(message: string) {
		return this.push('info', message);
	}

	dismiss(id: number) {
		const i = this.items.findIndex((t) => t.id === id);
		if (i >= 0) this.items.splice(i, 1);
	}
}

export const toasts = new ToastStore();
