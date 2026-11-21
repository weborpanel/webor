<script lang="ts">
	import { toasts, type ToastKind } from '#lib/stores/toasts.svelte.ts';
	import Icon from './Icon.svelte';
	import type { IconName } from '#lib/icons/paths.ts';

	const ICON: Record<ToastKind, IconName> = {
		success: 'check',
		error: 'alert',
		warn: 'alert',
		info: 'info'
	};
</script>

<div class="toasts" aria-live="polite" aria-atomic="false">
	{#each toasts.items as t (t.id)}
		<div class={['toast', t.kind]} role={t.kind === 'error' ? 'alert' : 'status'}>
			<span class="icon"><Icon name={ICON[t.kind]} size={16} /></span>
			<p>{t.message}</p>
			<button type="button" class="close" aria-label="Dismiss" onclick={() => toasts.dismiss(t.id)}>
				<Icon name="x" size={14} />
			</button>
		</div>
	{/each}
</div>

<style>
	.toasts {
		position: fixed;
		right: 1rem;
		bottom: 1rem;
		z-index: 100;
		display: grid;
		gap: 0.5rem;
		width: min(24rem, calc(100vw - 2rem));
		pointer-events: none;
	}

	.toast {
		pointer-events: auto;
		display: grid;
		grid-template-columns: auto 1fr auto;
		gap: 0.6rem;
		align-items: start;
		padding: 0.75rem 0.85rem;
		border-radius: var(--radius);
		background: var(--surface);
		border: 1px solid var(--border);
		border-left: 4px solid var(--info);
		box-shadow: var(--shadow-lg);
		animation: slide-in 0.18s ease-out;
	}

	.toast p {
		margin: 0;
		font-size: 0.88rem;
		font-weight: 600;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}

	.icon {
		color: var(--info);
		padding-top: 0.1rem;
	}

	.success {
		border-left-color: var(--ok);
	}

	.success .icon {
		color: var(--ok);
	}

	.error {
		border-left-color: var(--danger);
	}

	.error .icon {
		color: var(--danger);
	}

	.warn {
		border-left-color: var(--warn);
	}

	.warn .icon {
		color: var(--warn);
	}

	.close {
		background: none;
		border: 0;
		padding: 0.15rem;
		color: var(--subtle);
		cursor: pointer;
		border-radius: 4px;
	}

	.close:hover {
		color: var(--text);
		background: var(--surface-3);
	}

	@keyframes slide-in {
		from {
			transform: translateY(8px);
			opacity: 0;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.toast {
			animation: none;
		}
	}
</style>
