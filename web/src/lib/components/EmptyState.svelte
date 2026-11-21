<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';
	import type { IconName } from '#lib/icons/paths.ts';

	type Props = {
		icon?: IconName;
		title: string;
		description?: string;
		compact?: boolean;
		children?: Snippet;
	};

	let { icon = 'box', title, description, compact = false, children }: Props = $props();
</script>

<div class={['empty', compact && 'compact']}>
	<span class="badge"><Icon name={icon} size={compact ? 18 : 22} /></span>
	<p class="title">{title}</p>
	{#if description}<p class="desc">{description}</p>{/if}
	{#if children}<div class="actions">{@render children()}</div>{/if}
</div>

<style>
	.empty {
		display: grid;
		justify-items: center;
		text-align: center;
		gap: 0.35rem;
		padding: 2.25rem 1rem;
	}

	.compact {
		padding: 1rem 0.5rem;
		justify-items: start;
		text-align: left;
		grid-template-columns: auto 1fr;
		column-gap: 0.75rem;
		align-items: center;
	}

	.compact .desc,
	.compact .actions {
		grid-column: 2;
	}

	.badge {
		display: grid;
		place-items: center;
		width: 2.75rem;
		height: 2.75rem;
		border-radius: 12px;
		background: var(--accent-soft);
		color: var(--accent);
		margin-bottom: 0.35rem;
	}

	.compact .badge {
		width: 2.1rem;
		height: 2.1rem;
		margin: 0;
		grid-row: span 2;
	}

	.title {
		margin: 0;
		font-weight: 800;
	}

	.desc {
		margin: 0;
		color: var(--muted);
		font-size: 0.9rem;
		max-width: 28rem;
	}

	.actions {
		margin-top: 0.75rem;
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
	}

	.compact .actions {
		margin-top: 0.25rem;
	}
</style>
