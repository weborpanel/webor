<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { afterNavigate, goto } from '$app/navigation';
	import { page } from '$app/state';
	import { fetchAuthStatus, isProtectedPath, isPublicPath } from '#lib/auth';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	let authReady = $state(false);
	let authenticated = $state(false);
	let needsSetup = $state(false);
	let loggingOut = $state(false);

	let useDashboard = $derived(authReady && authenticated && !needsSetup && isProtectedPath(page.url.pathname));
	let sitesActive = $derived(page.url.pathname.startsWith('/sites'));

	async function refreshAuth() {
		try {
			const status = await fetchAuthStatus();
			authenticated = status.authenticated;
			needsSetup = status.needs_setup;
		} catch {
			authenticated = false;
			needsSetup = false;
		} finally {
			authReady = true;
		}
	}

	async function guard() {
		if (!authReady) return;
		const path = page.url.pathname;
		if (needsSetup && !path.startsWith('/setup')) {
			await goto('/setup');
			return;
		}
		if (!needsSetup && path.startsWith('/setup')) {
			await goto(authenticated ? '/sites' : '/login');
			return;
		}
		if (isProtectedPath(path) && !authenticated) {
			await goto('/login');
		}
	}

	async function logout() {
		loggingOut = true;
		try {
			await fetch('/api/auth/logout', { method: 'POST', credentials: 'same-origin' });
		} catch {
			// still clear local UI state
		}
		authenticated = false;
		loggingOut = false;
		await goto('/login');
	}

	onMount(async () => {
		await refreshAuth();
		await guard();
	});

	afterNavigate(async () => {
		await refreshAuth();
		await guard();
	});
</script>

<svelte:head>
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
	<link
		href="https://fonts.googleapis.com/css2?family=Nunito:wght@500;700;800&display=swap"
		rel="stylesheet"
	/>
	<link rel="icon" href="/icon.svg" type="image/svg+xml" />
	<meta name="description" content="Webor — lightweight non-Docker VPS hosting panel" />
</svelte:head>

{#if useDashboard}
	<div class="dash-shell">
		<aside class="sidebar">
			<a class="brand" href="/sites" aria-label="Webor home">
				<img class="logo" src="/logo-light.svg" width="160" height="52" alt="Webor" />
			</a>
			<nav class="side-nav" aria-label="Panel">
				<a class={['nav-item', sitesActive && 'active']} href="/sites" aria-current={sitesActive ? 'page' : undefined}>
					Sites
				</a>
			</nav>
			<div class="side-foot">
				<button type="button" class="logout" disabled={loggingOut} onclick={logout}>
					{loggingOut ? 'Signing out…' : 'Log out'}
				</button>
			</div>
		</aside>
		<main class="dash-main">
			{#if !authReady && isProtectedPath(page.url.pathname)}
				<p class="wbr-muted gate">Checking session…</p>
			{:else if isProtectedPath(page.url.pathname) && !authenticated}
				<p class="wbr-muted gate">Redirecting to log in…</p>
			{:else}
				{@render children()}
			{/if}
		</main>
	</div>
{:else}
	<div class="shell">
		<header class="top">
			<a class="brand" href="/" aria-label="Webor home">
				<img class="logo" src="/logo-light.svg" width="180" height="58" alt="Webor" />
			</a>
			<nav>
				{#if authReady && authenticated}
					<a href="/sites">Sites</a>
					<button type="button" class="linkish" disabled={loggingOut} onclick={logout}>
						{loggingOut ? 'Signing out…' : 'Log out'}
					</button>
				{:else if authReady && needsSetup}
					<a href="/setup">Setup</a>
				{:else}
					<a href="/login">Log in</a>
				{/if}
			</nav>
		</header>
		<main class="public-main">
			{#if !authReady && isProtectedPath(page.url.pathname)}
				<p class="wbr-muted gate">Checking session…</p>
			{:else if isProtectedPath(page.url.pathname) && !authenticated}
				<p class="wbr-muted gate">Redirecting to log in…</p>
			{:else if !isPublicPath(page.url.pathname) && needsSetup}
				<p class="wbr-muted gate">Redirecting to setup…</p>
			{:else}
				{@render children()}
			{/if}
		</main>
	</div>
{/if}

<style>
	.dash-shell {
		min-height: 100vh;
		display: grid;
		grid-template-columns: var(--sidebar-width) 1fr;
	}

	.sidebar {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
		padding: 1.35rem 1.1rem 1.5rem;
		background: color-mix(in srgb, var(--surface) 88%, transparent);
		border-right: 1px solid var(--border);
		position: sticky;
		top: 0;
		height: 100vh;
	}

	.brand {
		display: inline-flex;
		align-items: center;
		padding: 0 0.35rem;
	}

	.logo {
		display: block;
		height: auto;
		max-width: 100%;
	}

	.side-nav {
		display: grid;
		gap: 0.35rem;
		flex: 1;
	}

	.nav-item {
		display: block;
		padding: 0.65rem 0.85rem;
		border-radius: var(--radius-sm);
		font-weight: 800;
		font-size: 0.95rem;
		color: var(--text);
		text-decoration: none;
	}

	.nav-item:hover {
		background: color-mix(in srgb, var(--wbr-ultramarine) 8%, transparent);
		color: var(--text);
	}

	.nav-item.active {
		background: color-mix(in srgb, var(--wbr-ultramarine) 14%, transparent);
		color: var(--wbr-ultramarine);
	}

	.side-foot {
		padding-top: 0.5rem;
		border-top: 1px solid var(--border);
	}

	.logout {
		width: 100%;
		text-align: left;
		background: none;
		border: 0;
		padding: 0.65rem 0.85rem;
		border-radius: var(--radius-sm);
		font: inherit;
		font-weight: 700;
		color: var(--muted);
		cursor: pointer;
	}

	.logout:hover:not(:disabled) {
		color: var(--text);
		background: color-mix(in srgb, var(--text) 5%, transparent);
	}

	.logout:disabled {
		opacity: 0.7;
		cursor: wait;
	}

	.dash-main {
		min-width: 0;
		padding: var(--pane-pad);
		padding-bottom: 3rem;
	}

	.shell {
		min-height: 100vh;
		display: flex;
		flex-direction: column;
	}

	.top {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 1.25rem clamp(1rem, 4vw, 2.5rem);
	}

	.top nav {
		display: flex;
		gap: 1.1rem;
		align-items: center;
	}

	.top nav a,
	.linkish {
		font-weight: 700;
		font-size: 0.95rem;
	}

	.linkish {
		background: none;
		border: 0;
		padding: 0;
		color: inherit;
		font: inherit;
		font-weight: 700;
		cursor: pointer;
		text-decoration: underline;
		text-underline-offset: 0.15em;
	}

	.linkish:disabled {
		opacity: 0.7;
		cursor: wait;
	}

	.public-main {
		flex: 1;
		padding: 0 clamp(1rem, 4vw, 2.5rem) 3rem;
	}

	.gate {
		padding: 2rem 0;
	}

	@media (max-width: 720px) {
		.dash-shell {
			grid-template-columns: 1fr;
		}

		.sidebar {
			position: static;
			height: auto;
			border-right: 0;
			border-bottom: 1px solid var(--border);
			padding-bottom: 1rem;
		}

		.side-nav {
			grid-auto-flow: column;
			grid-auto-columns: max-content;
			overflow-x: auto;
			flex: 0;
		}

		.side-foot {
			border-top: 0;
			padding-top: 0;
		}

		.logout {
			width: auto;
		}
	}
</style>
