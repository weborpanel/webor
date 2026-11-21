<script lang="ts">
	import { page } from '$app/state';
	import Icon from './Icon.svelte';
	import { theme } from '#lib/stores/theme.svelte.ts';
	import { siteFromPath, siteSections } from '#lib/siteNav.ts';
	import { overview } from '#lib/stores/overview.svelte.ts';

	type Props = {
		email: string;
		loggingOut: boolean;
		onmenu: () => void;
		onsearch: () => void;
		onlogout: () => void;
	};

	let { email, loggingOut, onmenu, onsearch, onlogout }: Props = $props();

	let menuOpen = $state(false);

	const isMac = typeof navigator !== 'undefined' && /mac/i.test(navigator.platform || navigator.userAgent);

	type Crumb = { label: string; href?: string };

	let crumbs = $derived.by((): Crumb[] => {
		const path = page.url.pathname.replace(/\/+$/, '');
		if (path === '/dashboard') return [{ label: 'Dashboard' }];
		if (path === '/activity') return [{ label: 'Activity' }];
		if (path === '/account') return [{ label: 'Account' }];
		if (path === '/sites') return [{ label: 'Sites' }];
		const site = siteFromPath(path);
		if (site) {
			const type = overview.sites.find((s) => s.name.toLowerCase() === site.toLowerCase())?.type;
			const sections = siteSections(site, type);
			const base = sections[0].href;
			const section = sections.find((s) => s.key !== 'overview' && (path === s.href || path.startsWith(`${s.href}/`)));
			const out: Crumb[] = [{ label: 'Sites', href: '/sites' }, { label: site, href: section ? base : undefined }];
			if (section) out.push({ label: section.label });
			return out;
		}
		return [];
	});

	const THEME_LABEL = { system: 'System theme', light: 'Light theme', dark: 'Dark theme' } as const;
	const THEME_ICON = { system: 'monitor', light: 'sun', dark: 'moon' } as const;

	function closeMenuOnOutside(node: HTMLElement) {
		const handler = (e: MouseEvent) => {
			if (menuOpen && !node.contains(e.target as Node)) menuOpen = false;
		};
		document.addEventListener('click', handler);
		return () => document.removeEventListener('click', handler);
	}
</script>

<header class="topbar">
	<button type="button" class="icon-btn menu" aria-label="Open menu" onclick={onmenu}>
		<Icon name="menu" size={19} />
	</button>

	<nav class="crumbs" aria-label="Breadcrumb">
		<ol>
			{#each crumbs as c, i (i)}
				<li>
					{#if i > 0}<Icon name="chevronRight" size={14} class="sep" />{/if}
					{#if c.href && i < crumbs.length - 1}
						<a href={c.href}>{c.label}</a>
					{:else}
						<span aria-current={i === crumbs.length - 1 ? 'page' : undefined}>{c.label}</span>
					{/if}
				</li>
			{/each}
		</ol>
	</nav>

	<button type="button" class="search" onclick={onsearch}>
		<Icon name="search" size={15} />
		<span class="search-label">Search or jump to…</span>
		<span class="keys"><kbd>{isMac ? '⌘' : 'Ctrl'}</kbd><kbd>K</kbd></span>
	</button>

	<button
		type="button"
		class="icon-btn"
		aria-label={`${THEME_LABEL[theme.pref]} (click to change)`}
		title={THEME_LABEL[theme.pref]}
		onclick={() => theme.cycle()}
	>
		<Icon name={THEME_ICON[theme.pref]} size={17} />
	</button>

	<div class="user" {@attach closeMenuOnOutside}>
		<button
			type="button"
			class="avatar-btn"
			aria-haspopup="menu"
			aria-expanded={menuOpen}
			onclick={() => (menuOpen = !menuOpen)}
		>
			<span class="avatar" aria-hidden="true">{(email || 'A').slice(0, 1).toUpperCase()}</span>
			<span class="visually-hidden">Account menu</span>
			<Icon name="chevronDown" size={14} />
		</button>
		{#if menuOpen}
			<div class="menu-pop" role="menu">
				<div class="who">
					<span class="wbr-muted">Signed in as</span>
					<strong>{email || 'admin'}</strong>
				</div>
				<a role="menuitem" href="/account" onclick={() => (menuOpen = false)}>
					<Icon name="user" size={15} /> Account & security
				</a>
				<a role="menuitem" href="/activity" onclick={() => (menuOpen = false)}>
					<Icon name="activity" size={15} /> Activity log
				</a>
				<button role="menuitem" type="button" disabled={loggingOut} onclick={onlogout}>
					<Icon name="logout" size={15} />
					{loggingOut ? 'Signing out…' : 'Log out'}
				</button>
			</div>
		{/if}
	</div>
</header>

<style>
	.topbar {
		position: sticky;
		top: 0;
		z-index: 30;
		height: var(--topbar-h);
		display: flex;
		align-items: center;
		gap: 0.6rem;
		padding: 0 var(--pane-pad);
		background: color-mix(in srgb, var(--surface) 88%, transparent);
		backdrop-filter: blur(10px);
		border-bottom: 1px solid var(--border);
	}

	.icon-btn {
		display: grid;
		place-items: center;
		width: 2.1rem;
		height: 2.1rem;
		border-radius: var(--radius-sm);
		border: 1px solid transparent;
		background: none;
		color: var(--muted);
		cursor: pointer;
		flex-shrink: 0;
	}

	.icon-btn:hover {
		background: var(--surface-3);
		color: var(--text);
	}

	.menu {
		display: none;
	}

	.crumbs {
		flex: 1;
		min-width: 0;
	}

	ol {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		align-items: center;
		gap: 0.15rem;
		font-size: 0.9rem;
		font-weight: 700;
		min-width: 0;
	}

	li {
		display: flex;
		align-items: center;
		gap: 0.15rem;
		min-width: 0;
	}

	li :global(.sep) {
		color: var(--subtle);
	}

	li a {
		color: var(--muted);
		text-decoration: none;
		padding: 0.2rem 0.35rem;
		border-radius: 5px;
	}

	li a:hover {
		background: var(--surface-3);
		color: var(--text);
	}

	li span {
		padding: 0.2rem 0.35rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.search {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		width: min(17rem, 30vw);
		padding: 0.4rem 0.5rem 0.4rem 0.7rem;
		border-radius: var(--radius-sm);
		border: 1px solid var(--border);
		background: var(--surface-2);
		color: var(--subtle);
		font: inherit;
		font-size: 0.85rem;
		cursor: pointer;
	}

	.search:hover {
		border-color: var(--border-strong);
		color: var(--muted);
	}

	.search-label {
		flex: 1;
		text-align: left;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.keys {
		display: flex;
		gap: 2px;
	}

	.user {
		position: relative;
	}

	.avatar-btn {
		display: flex;
		align-items: center;
		gap: 0.3rem;
		background: none;
		border: 0;
		padding: 0.2rem;
		border-radius: 999px;
		cursor: pointer;
		color: var(--muted);
	}

	.avatar {
		display: grid;
		place-items: center;
		width: 2rem;
		height: 2rem;
		border-radius: 50%;
		background: var(--accent);
		color: var(--accent-contrast);
		font-weight: 800;
		font-size: 0.9rem;
	}

	.menu-pop {
		position: absolute;
		right: 0;
		top: calc(100% + 0.4rem);
		width: 15rem;
		padding: 0.35rem;
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		box-shadow: var(--shadow-lg);
		display: grid;
		gap: 2px;
	}

	.who {
		display: grid;
		padding: 0.5rem 0.6rem 0.6rem;
		border-bottom: 1px solid var(--border);
		margin-bottom: 0.25rem;
		font-size: 0.82rem;
		overflow-wrap: anywhere;
	}

	.menu-pop a,
	.menu-pop button {
		display: flex;
		align-items: center;
		gap: 0.55rem;
		padding: 0.5rem 0.6rem;
		border-radius: 6px;
		font: inherit;
		font-size: 0.88rem;
		font-weight: 600;
		color: var(--text);
		text-decoration: none;
		background: none;
		border: 0;
		cursor: pointer;
		text-align: left;
	}

	.menu-pop a:hover,
	.menu-pop button:hover:not(:disabled) {
		background: var(--surface-3);
	}

	@media (max-width: 960px) {
		.menu {
			display: grid;
		}
	}

	@media (max-width: 640px) {
		.search {
			width: auto;
			padding: 0.45rem;
			border: 0;
			background: none;
		}

		.search-label,
		.keys {
			display: none;
		}
	}
</style>
