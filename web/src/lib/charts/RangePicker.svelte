<script lang="ts">
	import type { MetricsRange } from '#lib/types.ts';

	type Props = {
		value: MetricsRange;
		onchange?: (v: MetricsRange) => void;
	};

	let { value = $bindable(), onchange }: Props = $props();

	const OPTIONS: { v: MetricsRange; label: string; title: string }[] = [
		{ v: '1h', label: '1H', title: 'Last hour' },
		{ v: '24h', label: '24H', title: 'Last 24 hours' },
		{ v: '7d', label: '7D', title: 'Last 7 days' }
	];
</script>

<div class="range" role="radiogroup" aria-label="Time range">
	{#each OPTIONS as o (o.v)}
		<button
			type="button"
			role="radio"
			aria-checked={value === o.v}
			title={o.title}
			class={[value === o.v && 'on']}
			onclick={() => {
				value = o.v;
				onchange?.(o.v);
			}}
		>
			{o.label}
		</button>
	{/each}
</div>

<style>
	.range {
		display: inline-flex;
		padding: 2px;
		border-radius: var(--radius-sm);
		background: var(--surface-3);
		border: 1px solid var(--border);
	}

	button {
		font: inherit;
		font-size: 0.76rem;
		font-weight: 800;
		letter-spacing: 0.03em;
		padding: 0.28rem 0.6rem;
		border: 0;
		border-radius: 5px;
		background: none;
		color: var(--muted);
		cursor: pointer;
	}

	button:hover {
		color: var(--text);
	}

	button.on {
		background: var(--surface);
		color: var(--accent);
		box-shadow: var(--shadow-sm);
	}
</style>
