<script lang="ts" module>
	export type Attention = {
		key: string;
		tone: 'danger' | 'warn' | 'info';
		title: string;
		detail?: string;
		href?: string;
		action?: string;
	};
</script>

<script lang="ts">
	import Icon from './Icon.svelte';

	type Props = { items: Attention[] };

	let { items }: Props = $props();
</script>

<ul class="attention">
	{#each items as item (item.key)}
		<li class={item.tone}>
			<span class="icon"><Icon name={item.tone === 'info' ? 'info' : 'alert'} size={16} /></span>
			<div class="text">
				<strong>{item.title}</strong>
				{#if item.detail}<span>{item.detail}</span>{/if}
			</div>
			{#if item.href}
				<a class="go" href={item.href}>{item.action ?? 'View'} <Icon name="chevronRight" size={14} /></a>
			{/if}
		</li>
	{/each}
</ul>

<style>
	.attention {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.5rem;
	}

	li {
		display: grid;
		grid-template-columns: auto 1fr auto;
		gap: 0.65rem;
		align-items: center;
		padding: 0.65rem 0.85rem;
		border-radius: var(--radius);
		border: 1px solid var(--border);
		background: var(--info-soft);
	}

	.icon {
		color: var(--info);
		display: grid;
	}

	li.warn {
		background: var(--warn-soft);
		border-color: color-mix(in srgb, var(--warn) 30%, var(--border));
	}

	li.warn .icon {
		color: var(--warn);
	}

	li.danger {
		background: var(--danger-soft);
		border-color: color-mix(in srgb, var(--danger) 30%, var(--border));
	}

	li.danger .icon {
		color: var(--danger);
	}

	.text {
		display: grid;
		min-width: 0;
		font-size: 0.88rem;
	}

	.text span {
		color: var(--muted);
		font-size: 0.82rem;
		overflow-wrap: anywhere;
	}

	.go {
		display: inline-flex;
		align-items: center;
		gap: 0.15rem;
		font-size: 0.82rem;
		font-weight: 700;
		text-decoration: none;
		white-space: nowrap;
	}
</style>
