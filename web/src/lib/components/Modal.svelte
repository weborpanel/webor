<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	type Props = {
		open: boolean;
		title: string;
		description?: string;
		width?: string;
		/** Prevent closing via Escape/backdrop (e.g. while submitting). */
		locked?: boolean;
		onclose?: () => void;
		children?: Snippet;
		footer?: Snippet;
	};

	let {
		open = $bindable(),
		title,
		description,
		width = '30rem',
		locked = false,
		onclose,
		children,
		footer
	}: Props = $props();

	const titleId = `modal-${Math.random().toString(36).slice(2, 8)}`;

	function sync(dialog: HTMLDialogElement) {
		if (open && !dialog.open) dialog.showModal();
		else if (!open && dialog.open) dialog.close();
	}

	function close() {
		if (locked) return;
		open = false;
	}
</script>

<dialog
	{@attach sync}
	class="modal"
	style:--modal-width={width}
	aria-labelledby={titleId}
	oncancel={(e) => {
		e.preventDefault();
		close();
	}}
	onclose={() => {
		open = false;
		onclose?.();
	}}
	onclick={(e) => {
		if (e.target === e.currentTarget) close();
	}}
>
	{#if open}
		<div class="panel">
			<header>
				<div>
					<h2 id={titleId}>{title}</h2>
					{#if description}<p>{description}</p>{/if}
				</div>
				<button type="button" class="close" aria-label="Close" disabled={locked} onclick={close}>
					<Icon name="x" size={16} />
				</button>
			</header>
			<div class="content">{@render children?.()}</div>
			{#if footer}<footer>{@render footer()}</footer>{/if}
		</div>
	{/if}
</dialog>

<style>
	.modal {
		padding: 0;
		border: 0;
		background: transparent;
		width: min(var(--modal-width), calc(100vw - 2rem));
		max-height: calc(100vh - 2rem);
		color: var(--text);
		overflow: visible;
	}

	.modal::backdrop {
		background: rgb(8 11 30 / 0.55);
		backdrop-filter: blur(2px);
	}

	.panel {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-lg);
		max-height: calc(100vh - 2rem);
		overflow: auto;
		animation: pop 0.16s ease-out;
	}

	header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 1rem;
		padding: 1.1rem 1.25rem 0;
	}

	h2 {
		margin: 0;
		font-size: 1.08rem;
		font-weight: 800;
	}

	header p {
		margin: 0.25rem 0 0;
		color: var(--muted);
		font-size: 0.88rem;
	}

	.close {
		background: none;
		border: 0;
		color: var(--subtle);
		padding: 0.3rem;
		border-radius: 6px;
		cursor: pointer;
	}

	.close:hover:not(:disabled) {
		background: var(--surface-3);
		color: var(--text);
	}

	.content {
		padding: 1rem 1.25rem 1.25rem;
	}

	footer {
		display: flex;
		justify-content: flex-end;
		flex-wrap: wrap;
		gap: 0.5rem;
		padding: 0.85rem 1.25rem;
		border-top: 1px solid var(--border);
		background: var(--surface-2);
		border-radius: 0 0 var(--radius-lg) var(--radius-lg);
	}

	@keyframes pop {
		from {
			transform: translateY(6px) scale(0.98);
			opacity: 0;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.panel {
			animation: none;
		}
	}
</style>
