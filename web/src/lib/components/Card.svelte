<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		title?: string;
		description?: string;
		id?: string;
		/** Remove body padding (tables, charts that bleed to the edge). */
		flush?: boolean;
		actions?: Snippet;
		footer?: Snippet;
		children?: Snippet;
		class?: string;
	};

	let { title, description, id, flush = false, actions, footer, children, class: className }: Props =
		$props();

	let headingId = $derived(id ? `${id}-title` : undefined);
</script>

<section class={['card', className]} {id} aria-labelledby={title ? headingId : undefined}>
	{#if title || actions}
		<header class="head">
			<div class="titles">
				{#if title}<h2 id={headingId}>{title}</h2>{/if}
				{#if description}<p>{description}</p>{/if}
			</div>
			{#if actions}<div class="actions">{@render actions()}</div>{/if}
		</header>
	{/if}
	<div class={['body', flush && 'flush']}>
		{@render children?.()}
	</div>
	{#if footer}
		<footer class="foot">{@render footer()}</footer>
	{/if}
</section>

<style>
	.card {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-sm);
		min-width: 0;
		display: flex;
		flex-direction: column;
	}

	.head {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		justify-content: space-between;
		gap: 0.75rem;
		padding: 1rem 1.25rem 0;
	}

	.titles {
		min-width: 0;
	}

	h2 {
		margin: 0;
		font-size: 0.98rem;
		font-weight: 800;
	}

	.titles p {
		margin: 0.2rem 0 0;
		color: var(--muted);
		font-size: 0.86rem;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.4rem;
	}

	.body {
		padding: 1rem 1.25rem 1.15rem;
		flex: 1;
		min-width: 0;
	}

	.body.flush {
		padding: 0.75rem 0 0;
		overflow-x: auto;
	}

	.foot {
		padding: 0.75rem 1.25rem;
		border-top: 1px solid var(--border);
		background: var(--surface-2);
		border-radius: 0 0 var(--radius-lg) var(--radius-lg);
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		align-items: center;
	}
</style>
