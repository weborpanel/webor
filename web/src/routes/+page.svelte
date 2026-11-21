<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { fetchAuthStatus, HOME_PATH } from '#lib/auth.ts';

	let panelHref = $state('/login');
	let panelLabel = $state('Log in');

	onMount(async () => {
		try {
			const status = await fetchAuthStatus();
			if (status.needs_setup) {
				panelHref = '/setup';
				panelLabel = 'Start setup';
				await goto('/setup');
				return;
			}
			if (status.authenticated) {
				panelHref = HOME_PATH;
				panelLabel = 'Open panel';
			}
		} catch {
			// keep the log in link
		}
	});
</script>

<svelte:head>
	<title>Webor</title>
</svelte:head>

<section class="hero">
	<img class="mark" src="/icon.svg" width="64" height="64" alt="" aria-hidden="true" />
	<h1>Host every site on one VPS.</h1>
	<p>
		Static builds, native apps, databases, caches and automatic SSL, without Docker. One small
		panel, everything in view.
	</p>
	<p class="cta">
		<a class="wbr-btn" href={panelHref}>{panelLabel}</a>
	</p>
	<ul class="points">
		<li>Live CPU, memory and disk per site</li>
		<li>Git or binary deploys with automatic rollback</li>
		<li>Postgres, MariaDB, SQLite and Valkey per site</li>
	</ul>
</section>

<style>
	.hero {
		margin: auto 0;
		max-width: 40rem;
		padding: 2rem 0 4rem;
	}

	.mark {
		margin-bottom: 1.25rem;
		border-radius: 16px;
	}

	h1 {
		margin: 0 0 0.85rem;
		font-size: clamp(2.2rem, 6vw, 3.6rem);
		line-height: 1.02;
		letter-spacing: -0.04em;
		font-weight: 800;
	}

	p {
		margin: 0;
		max-width: 46ch;
		font-size: clamp(1rem, 2.2vw, 1.15rem);
		color: var(--muted);
	}

	.cta {
		margin-top: 1.75rem;
	}

	.cta .wbr-btn {
		padding: 0.8rem 1.4rem;
		font-size: 1rem;
	}

	.points {
		margin: 2rem 0 0;
		padding: 0;
		list-style: none;
		display: grid;
		gap: 0.4rem;
		color: var(--muted);
		font-size: 0.95rem;
	}

	.points li::before {
		content: '✓';
		color: var(--ok);
		font-weight: 800;
		margin-right: 0.55rem;
	}
</style>
