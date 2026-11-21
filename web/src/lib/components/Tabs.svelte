<script lang="ts" module>
	import type { IconName } from '#lib/icons/paths.ts';

	export type TabItem = {
		href: string;
		label: string;
		icon?: IconName;
		/** Match the href exactly instead of as a prefix. */
		exact?: boolean;
		badge?: string | number;
	};
</script>

<script lang="ts">
	import { page } from '$app/state';
	import Icon from './Icon.svelte';

	type Props = {
		items: TabItem[];
		label: string;
	};

	let { items, label }: Props = $props();

	function isActive(item: TabItem, path: string) {
		const p = path.replace(/\/+$/, '');
		const h = item.href.replace(/\/+$/, '');
		return item.exact ? p === h : p === h || p.startsWith(`${h}/`);
	}
</script>

<nav class="tabs" aria-label={label}>
	<ul>
		{#each items as item (item.href)}
			{@const active = isActive(item, page.url.pathname)}
			<li>
				<a
					href={item.href}
					class={['tab', active && 'active']}
					aria-current={active ? 'page' : undefined}
				>
					{#if item.icon}<Icon name={item.icon} size={15} />{/if}
					{item.label}
					{#if item.badge != null && item.badge !== ''}<span class="count">{item.badge}</span>{/if}
				</a>
			</li>
		{/each}
	</ul>
</nav>

<style>
	.tabs {
		border-bottom: 1px solid var(--border);
		overflow-x: auto;
		scrollbar-width: none;
		margin-bottom: 1.25rem;
	}

	ul {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		gap: 0.25rem;
	}

	.tab {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		padding: 0.6rem 0.8rem;
		font-size: 0.88rem;
		font-weight: 700;
		color: var(--muted);
		text-decoration: none;
		white-space: nowrap;
		border-bottom: 2px solid transparent;
		margin-bottom: -1px;
		border-radius: 6px 6px 0 0;
	}

	.tab:hover {
		color: var(--text);
		background: var(--surface-3);
	}

	.tab.active {
		color: var(--accent);
		border-bottom-color: var(--accent);
	}

	.count {
		font-size: 0.7rem;
		padding: 0 0.4rem;
		border-radius: 999px;
		background: var(--surface-3);
		color: var(--muted);
	}

	.tab.active .count {
		background: var(--accent-soft);
		color: var(--accent);
	}
</style>
