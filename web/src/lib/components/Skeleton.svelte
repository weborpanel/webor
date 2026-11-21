<script lang="ts">
	type Props = {
		width?: string;
		height?: string;
		lines?: number;
		radius?: string;
	};

	let { width = '100%', height = '0.9rem', lines = 1, radius = '6px' }: Props = $props();
</script>

<span class="skeleton-group" role="status" aria-label="Loading">
	{#each { length: lines }, i (i)}
		<span
			class="skeleton"
			style:width={lines > 1 && i === lines - 1 ? '60%' : width}
			style:height
			style:border-radius={radius}
		></span>
	{/each}
</span>

<style>
	.skeleton-group {
		display: grid;
		gap: 0.5rem;
		width: 100%;
	}

	.skeleton {
		display: block;
		background: linear-gradient(
			90deg,
			var(--surface-3) 0%,
			color-mix(in srgb, var(--surface-3) 50%, var(--surface)) 50%,
			var(--surface-3) 100%
		);
		background-size: 200% 100%;
		animation: shimmer 1.4s ease-in-out infinite;
	}

	@keyframes shimmer {
		from {
			background-position: 200% 0;
		}
		to {
			background-position: -200% 0;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.skeleton {
			animation: none;
		}
	}
</style>
