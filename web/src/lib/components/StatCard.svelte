<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';
	import type { IconName } from '#lib/icons/paths.ts';
	import { usageLevel } from '#lib/format.ts';

	type Props = {
		label: string;
		value: string;
		sub?: string;
		icon?: IconName;
		/** 0–100; renders a usage bar coloured by severity. */
		pct?: number;
		loading?: boolean;
		children?: Snippet;
	};

	let { label, value, sub, icon, pct, loading = false, children }: Props = $props();

	let level = $derived(pct == null ? 'ok' : usageLevel(pct));
</script>

<article class="stat">
	<div class="top">
		<span class="label">{label}</span>
		{#if icon}<span class="icon"><Icon name={icon} size={16} /></span>{/if}
	</div>
	<div class="value" aria-busy={loading}>{loading ? '…' : value}</div>
	{#if sub}<div class="sub">{sub}</div>{/if}
	{#if pct != null}
		<div
			class="bar"
			role="progressbar"
			aria-label={`${label} usage`}
			aria-valuemin={0}
			aria-valuemax={100}
			aria-valuenow={Math.round(pct)}
		>
			<span class={['fill', level]} style:width={`${Math.max(pct, 1)}%`}></span>
		</div>
	{/if}
	{#if children}<div class="extra">{@render children()}</div>{/if}
</article>

<style>
	.stat {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-sm);
		padding: 0.95rem 1.1rem;
		display: grid;
		gap: 0.25rem;
		min-width: 0;
	}

	.top {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 0.5rem;
	}

	.label {
		font-size: 0.76rem;
		font-weight: 800;
		color: var(--subtle);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.icon {
		display: grid;
		place-items: center;
		width: 1.85rem;
		height: 1.85rem;
		border-radius: 8px;
		background: var(--accent-soft);
		color: var(--accent);
	}

	.value {
		font-size: 1.55rem;
		font-weight: 800;
		letter-spacing: -0.02em;
		font-variant-numeric: tabular-nums;
		line-height: 1.2;
	}

	.sub {
		font-size: 0.82rem;
		color: var(--muted);
	}

	.bar {
		margin-top: 0.4rem;
		height: 6px;
		border-radius: 999px;
		background: var(--surface-3);
		overflow: hidden;
	}

	.fill {
		display: block;
		height: 100%;
		border-radius: inherit;
		background: var(--ok);
		transition: width 0.4s ease;
	}

	.fill.ok {
		background: var(--accent);
	}

	.fill.warn {
		background: var(--warn);
	}

	.fill.danger {
		background: var(--danger);
	}

	.extra {
		margin-top: 0.35rem;
		min-width: 0;
	}
</style>
