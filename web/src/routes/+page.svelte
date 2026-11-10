<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	onMount(async () => {
		try {
			const res = await fetch('/api/auth/status', { credentials: 'same-origin' });
			const data = (await res.json().catch(() => ({}))) as {
				ok?: boolean;
				needs_setup?: boolean;
			};
			if (res.ok && data.ok && data.needs_setup) {
				await goto('/setup');
			}
		} catch {
			// Keep home available if status is unreachable
		}
	});
</script>

<svelte:head>
	<title>Webooza</title>
</svelte:head>

<section class="hero">
	<img class="mark" src="/icon.svg" width="72" height="72" alt="" aria-hidden="true" />
	<h1>Webooza</h1>
	<p>A lightweight hosting panel for one VPS and several sites — static builds, native apps, automatic SSL, without Docker.</p>
	<p class="cta">
		<a class="button" href="/sites">Open panel</a>
	</p>
</section>

<style>
	.hero {
		min-height: calc(100vh - 7rem);
		display: flex;
		flex-direction: column;
		justify-content: center;
		max-width: 38rem;
		padding-bottom: 4rem;
	}

	.mark {
		margin-bottom: 1.25rem;
		border-radius: 16px;
	}

	h1 {
		margin: 0 0 0.75rem;
		font-size: clamp(2.75rem, 8vw, 4.5rem);
		line-height: 0.95;
		letter-spacing: -0.04em;
		font-weight: 800;
	}

	p {
		margin: 0;
		max-width: 34ch;
		font-size: clamp(1.05rem, 2.4vw, 1.25rem);
		color: var(--muted);
	}

	.cta {
		margin-top: 1.75rem;
	}

	.button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.85rem 1.35rem;
		border-radius: 999px;
		background: var(--wbz-ultramarine);
		color: #fff;
		font-weight: 800;
		text-decoration: none;
		box-shadow: 0 10px 30px color-mix(in srgb, var(--wbz-ultramarine) 28%, transparent);
		transition: transform 160ms ease, box-shadow 160ms ease;
	}

	.button:hover {
		color: #fff;
		transform: translateY(-1px);
		box-shadow: 0 14px 34px color-mix(in srgb, var(--wbz-ultramarine) 36%, transparent);
	}
</style>
