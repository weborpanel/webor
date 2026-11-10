<script lang="ts">
	import '../app.css';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
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
			<a href="/sites">Sites</a>
			<a href="/login">Log in</a>
		</nav>
	</header>
	<main>
		{@render children()}
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

	nav a {
		font-weight: 700;
		font-size: 0.95rem;
	}

	main {
		flex: 1;
		padding: 0 clamp(1rem, 4vw, 2.5rem) 3rem;
	}
</style>
