<script lang="ts">
	type Props = {
		values: number[];
		color?: string;
		height?: number;
		/** Fixed max (e.g. 100 for percentages); defaults to the data max. */
		max?: number;
		label?: string;
	};

	let { values, color = 'var(--chart-1)', height = 32, max, label }: Props = $props();

	const uid = $props.id();
	const W = 100;

	let points = $derived.by(() => {
		const clean = values.filter((v) => v != null && !Number.isNaN(v));
		if (clean.length < 2) return '';
		const top = max ?? Math.max(...clean, 0.0001);
		const n = clean.length;
		return clean
			.map((v, i) => `${((i / (n - 1)) * W).toFixed(2)},${(height - 2 - (Math.min(v, top) / top) * (height - 4)).toFixed(2)}`)
			.join(' ');
	});
</script>

{#if points}
	<svg
		class="spark"
		viewBox={`0 0 ${W} ${height}`}
		preserveAspectRatio="none"
		style:height={`${height}px`}
		role={label ? 'img' : undefined}
		aria-label={label}
		aria-hidden={label ? undefined : 'true'}
	>
		<defs>
			<linearGradient id={`${uid}-fill`} x1="0" x2="0" y1="0" y2="1">
				<stop offset="0%" stop-color={color} stop-opacity="0.25" />
				<stop offset="100%" stop-color={color} stop-opacity="0" />
			</linearGradient>
		</defs>
		<polygon points={`0,${height} ${points} ${W},${height}`} fill={`url(#${uid}-fill)`} />
		<polyline
			{points}
			fill="none"
			stroke={color}
			stroke-width="1.6"
			stroke-linejoin="round"
			vector-effect="non-scaling-stroke"
		/>
	</svg>
{:else}
	<span class="flat" style:height={`${height}px`} aria-hidden="true"></span>
{/if}

<style>
	.spark {
		display: block;
		width: 100%;
	}

	.flat {
		display: block;
		width: 100%;
		border-bottom: 1px dashed var(--border-strong);
		transform: translateY(-50%);
	}
</style>
