<script lang="ts">
	type Segment = { key: string; label: string; value: number; color: string; href?: string };

	type Props = {
		segments: Segment[];
		/** Capacity; the remainder is drawn as free space. */
		total: number;
		format: (v: number) => string;
		label: string;
		/** Label for the remaining (unattributed) part. */
		restLabel?: string;
	};

	let { segments, total, format, label, restLabel = 'Other / free' }: Props = $props();

	let used = $derived(segments.reduce((a, s) => a + Math.max(s.value, 0), 0));
	let rest = $derived(Math.max(total - used, 0));
	let denom = $derived(Math.max(total, used, 1));
</script>

<div class="stacked">
	<div class="bar" role="img" aria-label={`${label}: ${segments.map((s) => `${s.label} ${format(s.value)}`).join(', ')}`}>
		{#each segments as s (s.key)}
			{#if s.value > 0}
				<span
					class="seg"
					style:width={`${(s.value / denom) * 100}%`}
					style:background={s.color}
					title={`${s.label}: ${format(s.value)}`}
				></span>
			{/if}
		{/each}
	</div>
	<ul class="legend">
		{#each segments as s (s.key)}
			<li>
				<span class="sw" style:background={s.color}></span>
				{#if s.href}<a href={s.href}>{s.label}</a>{:else}<span class="name">{s.label}</span>{/if}
				<strong>{format(s.value)}</strong>
				<span class="pct">{((s.value / denom) * 100).toFixed(1)}%</span>
			</li>
		{/each}
		{#if total > 0}
			<li class="rest">
				<span class="sw"></span>
				<span class="name">{restLabel}</span>
				<strong>{format(rest)}</strong>
				<span class="pct">{((rest / denom) * 100).toFixed(1)}%</span>
			</li>
		{/if}
	</ul>
</div>

<style>
	.stacked {
		display: grid;
		gap: 0.75rem;
	}

	.bar {
		display: flex;
		height: 12px;
		border-radius: 999px;
		overflow: hidden;
		background: var(--surface-3);
		gap: 2px;
	}

	.seg {
		display: block;
		height: 100%;
		min-width: 3px;
	}

	.legend {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.35rem;
		font-size: 0.85rem;
	}

	.legend li {
		display: grid;
		grid-template-columns: auto 1fr auto 3.5rem;
		gap: 0.55rem;
		align-items: center;
	}

	.sw {
		width: 0.7rem;
		height: 0.7rem;
		border-radius: 3px;
	}

	.rest .sw {
		background: var(--surface-3);
		border: 1px solid var(--border-strong);
	}

	.legend a,
	.name {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-weight: 600;
	}

	.legend a {
		color: var(--text);
		text-decoration: none;
	}

	.legend a:hover {
		color: var(--accent);
		text-decoration: underline;
	}

	.rest .name {
		color: var(--muted);
	}

	strong {
		font-variant-numeric: tabular-nums;
	}

	.pct {
		text-align: right;
		color: var(--subtle);
		font-variant-numeric: tabular-nums;
		font-size: 0.8rem;
	}
</style>
