<script lang="ts">
	import { usageLevel } from '#lib/format.ts';

	type Props = {
		/** 0–100, or undefined when there is no limit to compare against. */
		pct?: number;
		label: string;
		value: string;
		sub?: string;
		size?: number;
	};

	let { pct, label, value, sub, size = 104 }: Props = $props();

	const R = 42;
	const C = 2 * Math.PI * R;

	let clamped = $derived(pct == null ? 0 : Math.min(Math.max(pct, 0), 100));
	let level = $derived(pct == null ? 'none' : usageLevel(clamped));
</script>

<figure class="gauge">
	<div class="ring" style:width={`${size}px`} style:height={`${size}px`}>
		<svg
			viewBox="0 0 100 100"
			role="img"
			aria-label={`${label}: ${value}${pct != null ? `, ${Math.round(clamped)}% of limit` : ''}`}
		>
			<circle cx="50" cy="50" r={R} class="track" />
			{#if pct != null}
				<circle
					cx="50"
					cy="50"
					r={R}
					class={['arc', level]}
					stroke-dasharray={`${(clamped / 100) * C} ${C}`}
					transform="rotate(-90 50 50)"
				/>
			{/if}
		</svg>
		<div class="center">
			<strong>{pct != null ? `${Math.round(clamped)}%` : value}</strong>
		</div>
	</div>
	<figcaption>
		<span class="label">{label}</span>
		{#if pct != null}<span class="value">{value}</span>{/if}
		{#if sub}<span class="sub">{sub}</span>{/if}
	</figcaption>
</figure>

<style>
	.gauge {
		margin: 0;
		display: grid;
		justify-items: center;
		gap: 0.5rem;
		text-align: center;
	}

	.ring {
		position: relative;
	}

	svg {
		width: 100%;
		height: 100%;
		display: block;
	}

	circle {
		fill: none;
		stroke-width: 9;
	}

	.track {
		stroke: var(--surface-3);
	}

	.arc {
		stroke: var(--accent);
		stroke-linecap: round;
		transition: stroke-dasharray 0.5s ease;
	}

	.arc.warn {
		stroke: var(--warn);
	}

	.arc.danger {
		stroke: var(--danger);
	}

	.center {
		position: absolute;
		inset: 0;
		display: grid;
		place-items: center;
		font-size: 1.05rem;
		font-variant-numeric: tabular-nums;
	}

	figcaption {
		display: grid;
		gap: 0.05rem;
	}

	.label {
		font-size: 0.75rem;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--subtle);
	}

	.value {
		font-weight: 700;
		font-size: 0.9rem;
	}

	.sub {
		font-size: 0.78rem;
		color: var(--muted);
	}
</style>
