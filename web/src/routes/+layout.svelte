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
	<meta name="description" content="Webooza — lightweight non-Docker VPS hosting panel" />
</svelte:head>

<div class="shell">
	<header class="top">
		<a class="brand" href="/" aria-label="Webooza home">
			<picture>
				<source srcset="/logo-dark.svg" media="(prefers-color-scheme: dark)" />
				<img class="logo" src="/logo-light.svg" width="180" height="58" alt="Webooza" />
			</picture>
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
	<main>
		{#if !authReady && isProtectedPath(page.url.pathname)}
			<p class="muted">Checking session…</p>
		{:else if isProtectedPath(page.url.pathname) && !authenticated}
			<p class="muted">Redirecting to log in…</p>
		{:else if !isPublicPath(page.url.pathname) && needsSetup}
			<p class="muted">Redirecting to setup…</p>
		{:else}
			{@render children()}
		{/if}
	</main>
</div>

<style>
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

	.brand {
		display: inline-flex;
		align-items: center;
	}

	.logo {
		display: block;
		height: auto;
		max-width: min(180px, 48vw);
	}

	nav {
		display: flex;
		gap: 1.1rem;
		align-items: center;
	}

	nav a,
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

	main {
		flex: 1;
		padding: 0 clamp(1rem, 4vw, 2.5rem) 3rem;
	}

	.muted {
		color: var(--muted);
		padding: 2rem 0;
	}
</style>
