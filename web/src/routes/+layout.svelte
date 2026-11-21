<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { afterNavigate, goto } from '$app/navigation';
	import { page } from '$app/state';
	import { fetchAuthStatus, HOME_PATH, isProtectedPath, isPublicPath } from '#lib/auth.ts';
	import { request } from '#lib/api.ts';
	import { overview } from '#lib/stores/overview.svelte.ts';
	import { ui } from '#lib/stores/ui.svelte.ts';
	import Sidebar from '#lib/components/Sidebar.svelte';
	import Topbar from '#lib/components/Topbar.svelte';
	import CommandPalette from '#lib/components/CommandPalette.svelte';
	import AddSiteModal from '#lib/components/AddSiteModal.svelte';
	import Toasts from '#lib/components/Toasts.svelte';
	import ConfirmDialog from '#lib/components/ConfirmDialog.svelte';
	import Logo from '#lib/components/Logo.svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	let authReady = $state(false);
	let authenticated = $state(false);
	let needsSetup = $state(false);
	let loggingOut = $state(false);
	let email = $state('');

	let protectedRoute = $derived(isProtectedPath(page.url.pathname));
	let useShell = $derived(authReady && authenticated && !needsSetup && protectedRoute);

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
			await goto(authenticated ? HOME_PATH : '/login');
			return;
		}
		if (isProtectedPath(path) && !authenticated) {
			await goto('/login');
		}
	}

	async function loadSessionData() {
		if (!authenticated || needsSetup) return;
		void overview.ensure(30_000);
		if (!email) {
			try {
				const r = await request<{ email?: string }>('/api/me');
				if (r.ok) email = r.data.email || '';
			} catch {
				// non-fatal: topbar falls back to a generic label
			}
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
		email = '';
		overview.clear();
		await goto('/login');
	}

	function onkeydown(e: KeyboardEvent) {
		if (!useShell) return;
		if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
			e.preventDefault();
			ui.paletteOpen = !ui.paletteOpen;
		}
	}

	onMount(async () => {
		await refreshAuth();
		await guard();
		await loadSessionData();
	});

	afterNavigate(async () => {
		ui.drawerOpen = false;
		await refreshAuth();
		await guard();
		await loadSessionData();
	});
</script>

<svelte:window {onkeydown} />

<svelte:head>
	<link rel="icon" href="/icon.svg" type="image/svg+xml" />
	<meta name="description" content="Webor: lightweight non-Docker VPS hosting panel" />
</svelte:head>

{#if useShell}
	<a class="skip" href="#main">Skip to content</a>
	<div class="shell">
		<Sidebar bind:open={ui.drawerOpen} onaddsite={() => (ui.addSiteOpen = true)} />
		<div class="column">
			<Topbar
				{email}
				{loggingOut}
				onmenu={() => (ui.drawerOpen = true)}
				onsearch={() => (ui.paletteOpen = true)}
				onlogout={logout}
			/>
			<main id="main" class="main" tabindex="-1">
				{@render children()}
			</main>
		</div>
	</div>
	<CommandPalette bind:open={ui.paletteOpen} onaddsite={() => (ui.addSiteOpen = true)} />
	<AddSiteModal bind:open={ui.addSiteOpen} />
	<ConfirmDialog />
	<Toasts />
{:else}
	<div class="public">
		<header class="public-top">
			<a href="/" aria-label="Webor home"><Logo width={124} /></a>
			<nav>
				{#if authReady && authenticated && !needsSetup}
					<a class="wbr-btn sm" href={HOME_PATH}>Open panel</a>
				{:else if authReady && needsSetup}
					<a href="/setup">Setup</a>
				{:else if !page.url.pathname.startsWith('/login')}
					<a href="/login">Log in</a>
				{/if}
			</nav>
		</header>
		<main class="public-main">
			{#if !authReady && protectedRoute}
				<p class="wbr-muted gate">Checking session…</p>
			{:else if protectedRoute && !authenticated}
				<p class="wbr-muted gate">Redirecting to log in…</p>
			{:else if !isPublicPath(page.url.pathname) && needsSetup}
				<p class="wbr-muted gate">Redirecting to setup…</p>
			{:else}
				{@render children()}
			{/if}
		</main>
		<footer class="public-foot">Webor · lightweight hosting for one VPS</footer>
	</div>
	<Toasts />
{/if}

<style>
	.skip {
		position: absolute;
		left: -999px;
		top: 0.5rem;
		z-index: 200;
		padding: 0.5rem 0.8rem;
		background: var(--accent);
		color: var(--accent-contrast);
		border-radius: var(--radius-sm);
		font-weight: 700;
	}

	.skip:focus {
		left: 0.5rem;
	}

	.shell {
		min-height: 100vh;
	}

	.column {
		margin-left: var(--sidebar-width);
		min-width: 0;
		min-height: 100vh;
		display: flex;
		flex-direction: column;
	}

	.main {
		flex: 1;
		width: 100%;
		max-width: 88rem;
		padding: 1.5rem var(--pane-pad) 4rem;
		outline: none;
		min-width: 0;
	}

	.public {
		min-height: 100vh;
		display: flex;
		flex-direction: column;
		background:
			radial-gradient(
				900px 480px at 10% -10%,
				color-mix(in srgb, var(--wbr-ultramarine) 12%, transparent),
				transparent 60%
			),
			radial-gradient(
				700px 380px at 100% 0%,
				color-mix(in srgb, var(--wbr-marigold) 10%, transparent),
				transparent 55%
			),
			var(--page);
	}

	.public-top {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 1.25rem clamp(1rem, 4vw, 2.5rem);
	}

	.public-top nav {
		display: flex;
		gap: 1rem;
		align-items: center;
		font-weight: 700;
	}

	.public-main {
		flex: 1;
		display: flex;
		flex-direction: column;
		padding: 0 clamp(1rem, 4vw, 2.5rem) 3rem;
	}

	.public-foot {
		padding: 1.25rem;
		text-align: center;
		font-size: 0.8rem;
		color: var(--subtle);
	}

	.gate {
		padding: 2rem 0;
	}

	@media (max-width: 960px) {
		.column {
			margin-left: 0;
		}
	}
</style>
