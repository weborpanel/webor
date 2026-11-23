<script lang="ts">
	import { page } from '$app/state';
	import Icon from './Icon.svelte';
	import Logo from './Logo.svelte';
	import type { IconName } from '#lib/icons/paths.ts';
	import { overview } from '#lib/stores/overview.svelte.ts';
	import { siteFromPath, siteSections } from '#lib/siteNav.ts';
	import { siteStatus } from '#lib/siteStatus.ts';
	import type { OverviewSite } from '#lib/types.ts';

	type Props = {
		/** Mobile drawer state. */
		open: boolean;
		onaddsite: () => void;
	};

	let { open = $bindable(), onaddsite }: Props = $props();

	let filter = $state('');

	type NavItem = { href: string; label: string; icon: IconName; external?: boolean };

	const NAV: NavItem[] = [
		{ href: '/dashboard', label: 'Dashboard', icon: 'dashboard' },
		{ href: '/sites', label: 'Sites', icon: 'globe' },
		{ href: '/backups', label: 'Backups', icon: 'server' },
		{ href: '/activity', label: 'Activity', icon: 'activity' },
		{ href: '/db/', label: 'Databases', icon: 'database', external: true }
	];

	let path = $derived(page.url.pathname);
	let currentSite = $derived(siteFromPath(path).toLowerCase());

	/** Top-level sites with their subdomains grouped underneath. */
	let groups = $derived.by(() => {
		const sites = overview.sites;
		const q = filter.trim().toLowerCase();
		const byParent: Record<string, OverviewSite[]> = {};
		for (const s of sites) {
			if (!s.parent) continue;
			const key = s.parent.toLowerCase();
			(byParent[key] ??= []).push(s);
		}
		const names = new Set(sites.map((s) => s.name.toLowerCase()));
		return sites
			.filter((s) => !s.parent || !names.has(s.parent.toLowerCase()))
			.map((s) => ({ site: s, children: byParent[s.name.toLowerCase()] ?? [] }))
			.filter(
				(g) =>
					!q ||
					g.site.name.toLowerCase().includes(q) ||
					g.children.some((c) => c.name.toLowerCase().includes(q))
			)
			.sort((a, b) => a.site.name.localeCompare(b.site.name));
	});

	function navActive(item: NavItem) {
		if (item.external) return false;
		if (item.href === '/sites') return path === '/sites';
		return path === item.href || path.startsWith(`${item.href}/`);
	}

	function sectionActive(href: string, exact?: boolean) {
		const p = path.replace(/\/+$/, '');
		return exact ? p === href : p === href || p.startsWith(`${href}/`);
	}
</script>

{#snippet siteLink(site: OverviewSite, nested: boolean)}
	{@const st = siteStatus(site)}
	{@const isCurrent = site.name.toLowerCase() === currentSite}
	<a
		href={`/sites/${encodeURIComponent(site.name)}`}
		class={['site-link', nested && 'nested', isCurrent && 'current']}
		aria-current={isCurrent && path === `/sites/${encodeURIComponent(site.name)}` ? 'page' : undefined}
		title={`${site.name} · ${st.label}`}
	>
		<span class={['dot', st.tone]} aria-hidden="true"></span>
		<span class="site-name">{site.name}</span>
		{#if site.type === 'app'}<span class="tag">app</span>{/if}
	</a>
	{#if isCurrent}
		<ul class="sections">
			{#each siteSections(site.name, site.type) as sec (sec.key)}
				{@const active = sectionActive(sec.href, sec.exact)}
				<li>
					<a href={sec.href} class={['section-link', active && 'active']} aria-current={active ? 'page' : undefined}>
						{#if sec.icon}<Icon name={sec.icon} size={14} />{/if}
						{sec.label}
					</a>
				</li>
			{/each}
		</ul>
	{/if}
{/snippet}

<aside class={['sidebar', open && 'open']} aria-label="Primary">
	<div class="brand">
		<a href="/dashboard" aria-label="Webor dashboard"><Logo width={98} /></a>
		<button type="button" class="close-drawer" aria-label="Close menu" onclick={() => (open = false)}>
			<Icon name="x" size={18} />
		</button>
	</div>

	<nav class="primary" aria-label="Panel">
		<ul>
			{#each NAV as item (item.href)}
				{@const active = navActive(item)}
				<li>
					<a
						href={item.href}
						class={['nav-link', active && 'active']}
						aria-current={active ? 'page' : undefined}
						target={item.external ? '_blank' : undefined}
						rel={item.external ? 'noopener' : undefined}
					>
						<Icon name={item.icon} size={17} />
						<span>{item.label}</span>
						{#if item.external}<Icon name="external" size={13} class="ext" />{/if}
						{#if item.href === '/sites' && overview.sites.length}
							<span class="count">{overview.sites.length}</span>
						{/if}
					</a>
				</li>
			{/each}
		</ul>
	</nav>

	<div class="sites-head">
		<span>Sites</span>
		<button type="button" class="add" aria-label="Add site" title="Add site" onclick={onaddsite}>
			<Icon name="plus" size={15} />
		</button>
	</div>
	{#if overview.sites.length > 6}
		<label class="filter">
			<Icon name="search" size={14} />
			<span class="visually-hidden">Filter sites</span>
			<input bind:value={filter} placeholder="Filter sites" autocomplete="off" />
		</label>
	{/if}
	<nav class="sites" aria-label="Sites">
		{#if overview.loading && !overview.data}
			<p class="hint">Loading…</p>
		{:else if groups.length === 0}
			<p class="hint">{filter ? 'No matches' : 'No sites yet'}</p>
		{:else}
			<ul>
				{#each groups as g (g.site.name)}
					<li>
						{@render siteLink(g.site, false)}
						{#each g.children as child (child.name)}
							{@render siteLink(child, true)}
						{/each}
					</li>
				{/each}
			</ul>
		{/if}
	</nav>

	<div class="foot">
		<a href="/account" class={['nav-link', path.startsWith('/account') && 'active']}>
			<Icon name="user" size={17} />
			<span>Account</span>
		</a>
		{#if overview.data?.version}
			<span class="version">Webor {overview.data.version}</span>
		{/if}
	</div>
</aside>
<button
	type="button"
	class={['scrim', open && 'show']}
	aria-label="Close menu"
	tabindex="-1"
	onclick={() => (open = false)}
></button>

<style>
	.sidebar {
		position: fixed;
		inset: 0 auto 0 0;
		width: var(--sidebar-width);
		display: flex;
		flex-direction: column;
		background: var(--surface);
		border-right: 1px solid var(--border);
		z-index: 40;
		overflow: hidden;
	}

	.brand {
		display: flex;
		align-items: center;
		justify-content: space-between;
		height: var(--topbar-h);
		padding: 0 1rem;
		border-bottom: 1px solid var(--border);
		flex-shrink: 0;
	}

	.brand a {
		display: inline-flex;
	}

	.close-drawer {
		display: none;
		background: none;
		border: 0;
		color: var(--muted);
		cursor: pointer;
		padding: 0.3rem;
		border-radius: 6px;
	}

	ul {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.primary {
		padding: 0.75rem 0.6rem 0.5rem;
		flex-shrink: 0;
	}

	.primary ul {
		display: grid;
		gap: 2px;
	}

	.nav-link {
		display: flex;
		align-items: center;
		gap: 0.65rem;
		padding: 0.5rem 0.65rem;
		border-radius: var(--radius-sm);
		color: var(--muted);
		font-weight: 700;
		font-size: 0.9rem;
		text-decoration: none;
	}

	.nav-link:hover {
		background: var(--surface-3);
		color: var(--text);
	}

	.nav-link.active {
		background: var(--accent-soft);
		color: var(--accent);
	}

	.nav-link :global(.ext) {
		margin-left: auto;
		opacity: 0.6;
	}

	.count {
		margin-left: auto;
		font-size: 0.72rem;
		padding: 0 0.45rem;
		border-radius: 999px;
		background: var(--surface-3);
		color: var(--muted);
	}

	.sites-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.75rem 1.25rem 0.35rem;
		font-size: 0.7rem;
		font-weight: 800;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--subtle);
		border-top: 1px solid var(--border);
		margin-top: 0.25rem;
	}

	.add {
		display: grid;
		place-items: center;
		width: 1.5rem;
		height: 1.5rem;
		border-radius: 6px;
		border: 1px solid var(--border-strong);
		background: var(--surface);
		color: var(--muted);
		cursor: pointer;
	}

	.add:hover {
		color: var(--accent);
		border-color: var(--accent);
	}

	.filter {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		margin: 0.25rem 0.75rem 0.35rem;
		padding: 0.3rem 0.55rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-2);
		color: var(--subtle);
	}

	.filter input {
		flex: 1;
		min-width: 0;
		border: 0;
		background: none;
		font: inherit;
		font-size: 0.85rem;
		color: var(--text);
		outline: none;
	}

	.sites {
		flex: 1;
		overflow-y: auto;
		padding: 0.15rem 0.6rem 0.75rem;
		scrollbar-width: thin;
	}

	.hint {
		margin: 0.25rem 0.65rem;
		font-size: 0.85rem;
		color: var(--subtle);
	}

	.site-link {
		display: flex;
		align-items: center;
		gap: 0.55rem;
		padding: 0.4rem 0.65rem;
		border-radius: var(--radius-sm);
		color: var(--text);
		font-size: 0.86rem;
		font-weight: 600;
		text-decoration: none;
		min-width: 0;
	}

	.site-link.nested {
		padding-left: 1.5rem;
		font-size: 0.83rem;
		color: var(--muted);
	}

	.site-link:hover {
		background: var(--surface-3);
		color: var(--text);
	}

	.site-link.current {
		background: var(--surface-3);
		font-weight: 800;
	}

	.site-name {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.tag {
		font-size: 0.66rem;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--subtle);
	}

	.dot {
		width: 0.45rem;
		height: 0.45rem;
		border-radius: 50%;
		background: var(--subtle);
		flex-shrink: 0;
	}

	.dot.ok {
		background: var(--ok);
	}

	.dot.warn {
		background: var(--warn);
	}

	.dot.danger {
		background: var(--danger);
	}

	.dot.info {
		background: var(--info);
	}

	.sections {
		margin: 0.15rem 0 0.4rem 1.05rem;
		padding-left: 0.6rem;
		border-left: 1px solid var(--border-strong);
		display: grid;
		gap: 1px;
	}

	.section-link {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.3rem 0.55rem;
		border-radius: 6px;
		font-size: 0.82rem;
		font-weight: 600;
		color: var(--muted);
		text-decoration: none;
	}

	.section-link:hover {
		color: var(--text);
		background: var(--surface-3);
	}

	.section-link.active {
		color: var(--accent);
		background: var(--accent-soft);
	}

	.foot {
		border-top: 1px solid var(--border);
		padding: 0.6rem;
		display: grid;
		gap: 0.25rem;
		flex-shrink: 0;
	}

	.version {
		font-size: 0.72rem;
		color: var(--subtle);
		padding: 0 0.65rem;
	}

	.scrim {
		display: none;
	}

	@media (max-width: 960px) {
		.sidebar {
			transform: translateX(-100%);
			transition: transform 0.2s ease;
			box-shadow: var(--shadow-lg);
			width: min(18rem, 85vw);
		}

		.sidebar.open {
			transform: none;
		}

		.close-drawer {
			display: inline-flex;
		}

		.scrim {
			display: block;
			position: fixed;
			inset: 0;
			z-index: 39;
			border: 0;
			background: rgb(8 11 30 / 0.45);
			opacity: 0;
			pointer-events: none;
			transition: opacity 0.2s;
		}

		.scrim.show {
			opacity: 1;
			pointer-events: auto;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.sidebar,
		.scrim {
			transition: none;
		}
	}
</style>
