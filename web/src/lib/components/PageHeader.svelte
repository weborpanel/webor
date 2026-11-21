<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		title: string;
		subtitle?: string;
		meta?: Snippet;
		actions?: Snippet;
		children?: Snippet;
	};

	let { title, subtitle, meta, actions, children }: Props = $props();
</script>

<header class="page-header">
	<div class="text">
		<div class="title-row">
			<h1>{title}</h1>
			{@render meta?.()}
		</div>
		{#if subtitle}
			<p class="subtitle">{subtitle}</p>
		{/if}
		{@render children?.()}
	</div>
	{#if actions}
		<div class="actions">{@render actions()}</div>
	{/if}
</header>

<style>
	.page-header {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-end;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1.25rem;
	}

	.text {
		min-width: 0;
	}

	.title-row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.6rem;
	}

	h1 {
		margin: 0;
		font-size: clamp(1.4rem, 2.6vw, 1.75rem);
		font-weight: 800;
		overflow-wrap: anywhere;
	}

	.subtitle {
		margin: 0.3rem 0 0;
		color: var(--muted);
		font-size: 0.93rem;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
</style>
