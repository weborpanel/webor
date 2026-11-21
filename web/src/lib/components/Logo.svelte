<script lang="ts">
	type Props = { width?: number; compact?: boolean };

	let { width = 104, compact = false }: Props = $props();
</script>

{#if compact}
	<img class="mark" src="/icon.svg" width="30" height="30" alt="Webor" />
{:else}
	<span class="logo" style:width={`${width}px`}>
		<img class="light" src="/logo-light.svg" {width} alt="Webor" />
		<img class="dark" src="/logo-dark.svg" {width} alt="" aria-hidden="true" />
	</span>
{/if}

<style>
	.logo {
		display: inline-block;
		line-height: 0;
	}

	img {
		display: block;
		height: auto;
	}

	.dark {
		display: none;
	}

	:global(:root[data-theme='dark']) .light {
		display: none;
	}

	:global(:root[data-theme='dark']) .dark {
		display: block;
	}

	@media (prefers-color-scheme: dark) {
		:global(:root:not([data-theme='light'])) .light {
			display: none;
		}

		:global(:root:not([data-theme='light'])) .dark {
			display: block;
		}
	}

	.mark {
		display: block;
		border-radius: 8px;
	}
</style>
