<script lang="ts">
	import { ICONS, type IconName } from '#lib/icons/paths.ts';

	type Props = {
		name: IconName;
		size?: number;
		label?: string;
		class?: string;
	};

	let { name, size = 18, label, class: className }: Props = $props();
</script>

<svg
	class={['icon', className]}
	width={size}
	height={size}
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="2"
	stroke-linecap="round"
	stroke-linejoin="round"
	role={label ? 'img' : undefined}
	aria-label={label}
	aria-hidden={label ? undefined : 'true'}
>
	<path d={ICONS[name]} />
</svg>

<style>
	.icon {
		flex-shrink: 0;
		display: inline-block;
		vertical-align: middle;
	}
</style>
