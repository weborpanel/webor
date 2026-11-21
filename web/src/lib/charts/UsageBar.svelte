<script lang="ts">
	import { pctOf, usageLevel } from '#lib/format.ts';

	type Props = {
		label?: string;
		used: number;
		/** Limit or capacity; when 0/undefined only the used value is shown. */
		total?: number;
		format: (v: number) => string;
		/** Text when there is no total (e.g. "no limit"). */
		noTotalText?: string;
		compact?: boolean;
	};

	let { label, used, total, format, noTotalText = 'no limit', compact = false }: Props = $props();

	let pct = $derived(total ? pctOf(used, total) : 0);
	let level = $derived(usageLevel(pct));
</script>

<div class={['usage', compact && 'compact']}>
	{#if label || !compact}
		<div class="meta">
			{#if label}<span class="label">{label}</span>{/if}
			<span class="nums">
				<strong>{format(used)}</strong>
				{#if total}
					<span class="of">/ {format(total)}</span>
					<span class={['pct', level]}>{Math.round(pct)}%</span>
				{:else}
					<span class="of">{noTotalText}</span>
				{/if}
			</span>
		</div>
	{/if}
	<div
		class="track"
		role="progressbar"
		aria-label={label ?? 'Usage'}
		aria-valuemin={0}
		aria-valuemax={100}
		aria-valuenow={total ? Math.round(pct) : undefined}
		aria-valuetext={total ? `${format(used)} of ${format(total)}` : `${format(used)}, ${noTotalText}`}
	>
		<span class={['fill', level, !total && 'unbounded']} style:width={total ? `${Math.max(pct, 1.5)}%` : '100%'}></span>
	</div>
</div>

<style>
	.usage {
		display: grid;
		gap: 0.35rem;
		min-width: 0;
	}

	.meta {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		gap: 0.5rem;
		font-size: 0.84rem;
	}

	.label {
		font-weight: 700;
		color: var(--muted);
	}

	.nums {
		display: inline-flex;
		gap: 0.3rem;
		align-items: baseline;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}

	.of {
		color: var(--subtle);
	}

	.pct {
		font-size: 0.75rem;
		font-weight: 800;
		color: var(--accent);
	}

	.pct.warn {
		color: var(--warn);
	}

	.pct.danger {
		color: var(--danger);
	}

	.track {
		height: 7px;
		border-radius: 999px;
		background: var(--surface-3);
		overflow: hidden;
	}

	.compact .track {
		height: 5px;
	}

	.fill {
		display: block;
		height: 100%;
		border-radius: inherit;
		background: var(--accent);
		transition: width 0.4s ease;
	}

	.fill.warn {
		background: var(--warn);
	}

	.fill.danger {
		background: var(--danger);
	}

	.fill.unbounded {
		background: repeating-linear-gradient(
			-45deg,
			color-mix(in srgb, var(--accent) 35%, transparent) 0 6px,
			color-mix(in srgb, var(--accent) 15%, transparent) 6px 12px
		);
	}
</style>
