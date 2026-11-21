<script lang="ts" module>
	export type ChartSeries = {
		key: string;
		label: string;
		values: number[];
		color: string;
		/** Fill under the line (default true for the first series). */
		area?: boolean;
		dashed?: boolean;
	};
</script>

<script lang="ts">
	import { formatAxisTime, formatTooltipTime } from '#lib/format.ts';
	import {
		areaPath,
		formatTick,
		formatValue,
		linePath,
		nearestIndex,
		segments,
		yTicks,
		type ValueKind
	} from './scale.ts';

	type Props = {
		t: number[];
		series: ChartSeries[];
		/** Seconds between samples; gaps over ~2.5 steps break the line. */
		step: number;
		from: number;
		to: number;
		kind?: ValueKind;
		/** Fixed y maximum (otherwise fitted to data). */
		yMax?: number;
		threshold?: { value: number; label: string };
		height?: number;
		label: string;
		emptyText?: string;
	};

	let {
		t,
		series,
		step,
		from,
		to,
		kind = 'number',
		yMax,
		threshold,
		height = 220,
		label,
		emptyText = 'No samples yet. Data appears about a minute after the panel starts.'
	}: Props = $props();

	const uid = $props.id();

	const M = { top: 12, right: 14, bottom: 26, left: 56 };

	let width = $state(0);
	let hoverIndex = $state(-1);

	let innerW = $derived(Math.max(width - M.left - M.right, 10));
	let innerH = $derived(height - M.top - M.bottom);

	let dataMax = $derived.by(() => {
		let m = threshold?.value ?? 0;
		for (const s of series) for (const v of s.values) if (v > m) m = v;
		return m;
	});

	let ticks = $derived(yTicks(kind, yMax ?? (dataMax > 0 ? dataMax * 1.08 : kind === 'pct' ? 10 : 1)));
	let top = $derived(yMax ?? ticks[ticks.length - 1] ?? 1);

	let span = $derived(Math.max(to - from, 1));
	const x = (ts: number) => M.left + ((ts - from) / span) * innerW;
	const y = (v: number) => M.top + innerH - (Math.min(v, top) / top) * innerH;

	let xTicks = $derived.by(() => {
		const n = width < 480 ? 3 : 5;
		return Array.from({ length: n }, (_, i) => from + (span * (i + 0.5)) / n);
	});

	let paths = $derived(
		series.map((s, si) => {
			const segs = segments(t, s.values, step * 2.5);
			const pts = segs.map((seg) => seg.map((i) => ({ x: x(t[i]), y: y(s.values[i]) })));
			const showArea = s.area ?? si === 0;
			return {
				...s,
				line: pts.map(linePath).join(''),
				area: showArea ? pts.map((p) => areaPath(p, M.top + innerH)).join('') : ''
			};
		})
	);

	let latest = $derived(
		series.map((s) => {
			for (let i = s.values.length - 1; i >= 0; i--) if (s.values[i] != null) return s.values[i];
			return undefined;
		})
	);

	let ariaLabel = $derived(
		`${label}. ` +
			series
				.map((s, i) => (latest[i] != null ? `${s.label} latest ${formatValue(kind, latest[i]!)}` : ''))
				.filter(Boolean)
				.join(', ')
	);

	function onpointermove(e: PointerEvent) {
		const rect = (e.currentTarget as SVGElement).getBoundingClientRect();
		const px = e.clientX - rect.left;
		if (px < M.left || px > M.left + innerW) {
			hoverIndex = -1;
			return;
		}
		const ts = from + ((px - M.left) / innerW) * span;
		hoverIndex = nearestIndex(t, ts);
	}

	let hover = $derived.by(() => {
		if (hoverIndex < 0 || hoverIndex >= t.length) return null;
		const ts = t[hoverIndex];
		const hx = x(ts);
		return {
			x: hx,
			ts,
			flip: hx > M.left + innerW * 0.6,
			rows: series.map((s) => ({ label: s.label, color: s.color, v: s.values[hoverIndex] }))
		};
	});
</script>

<div class="chart" bind:clientWidth={width} style:height={`${height}px`}>
	{#if t.length === 0}
		<div class="empty">{emptyText}</div>
	{/if}
	{#if width > 0}
		<svg
			{width}
			{height}
			role="img"
			aria-label={ariaLabel}
			{onpointermove}
			onpointerleave={() => (hoverIndex = -1)}
		>
			<defs>
				{#each paths as p, i (p.key)}
					<linearGradient id={`${uid}-g${i}`} x1="0" x2="0" y1="0" y2="1">
						<stop offset="0%" stop-color={p.color} stop-opacity="0.28" />
						<stop offset="100%" stop-color={p.color} stop-opacity="0.02" />
					</linearGradient>
				{/each}
			</defs>

			{#each ticks as tv (tv)}
				<line class="grid" x1={M.left} x2={M.left + innerW} y1={y(tv)} y2={y(tv)} />
				<text class="ylabel" x={M.left - 8} y={y(tv)} dy="0.32em" text-anchor="end">
					{formatTick(kind, tv)}
				</text>
			{/each}

			{#each xTicks as xt (xt)}
				<text class="xlabel" x={x(xt)} y={height - 6} text-anchor="middle">
					{formatAxisTime(xt, span)}
				</text>
			{/each}

			{#if threshold && threshold.value <= top}
				<line
					class="threshold"
					x1={M.left}
					x2={M.left + innerW}
					y1={y(threshold.value)}
					y2={y(threshold.value)}
				/>
				<text class="threshold-label" x={M.left + innerW} y={y(threshold.value) - 5} text-anchor="end">
					{threshold.label}
				</text>
			{/if}

			{#each paths as p, i (p.key)}
				{#if p.area}
					<path d={p.area} fill={`url(#${uid}-g${i})`} stroke="none" />
				{/if}
				<path
					d={p.line}
					fill="none"
					stroke={p.color}
					stroke-width="2"
					stroke-linejoin="round"
					stroke-linecap="round"
					stroke-dasharray={p.dashed ? '4 4' : undefined}
				/>
			{/each}

			{#if hover}
				<line class="cross" x1={hover.x} x2={hover.x} y1={M.top} y2={M.top + innerH} />
				{#each hover.rows as r (r.label)}
					{#if r.v != null}
						<circle cx={hover.x} cy={y(r.v)} r="4" fill={r.color} stroke="var(--surface)" stroke-width="2" />
					{/if}
				{/each}
			{/if}
		</svg>
	{/if}

	{#if hover}
		<div
			class="tooltip"
			style:left={hover.flip ? 'auto' : `${hover.x + 12}px`}
			style:right={hover.flip ? `${width - hover.x + 12}px` : 'auto'}
			style:top={`${M.top}px`}
		>
			<div class="tt-time">{formatTooltipTime(hover.ts)}</div>
			{#each hover.rows as r (r.label)}
				<div class="tt-row">
					<span class="swatch" style:background={r.color}></span>
					<span class="tt-label">{r.label}</span>
					<strong>{r.v != null ? formatValue(kind, r.v) : '—'}</strong>
				</div>
			{/each}
		</div>
	{/if}
</div>

{#if series.length > 1}
	<ul class="legend">
		{#each series as s, i (s.key)}
			<li>
				<span class={['swatch', s.dashed && 'dashed']} style:background={s.color}></span>
				{s.label}
				{#if latest[i] != null}<strong>{formatValue(kind, latest[i]!)}</strong>{/if}
			</li>
		{/each}
	</ul>
{/if}

<style>
	.chart {
		position: relative;
		width: 100%;
		min-width: 0;
		user-select: none;
	}

	svg {
		display: block;
		overflow: visible;
		touch-action: pan-y;
	}

	.grid {
		stroke: var(--chart-grid);
		stroke-width: 1;
	}

	.ylabel,
	.xlabel {
		font-size: 11px;
		fill: var(--subtle);
		font-variant-numeric: tabular-nums;
	}

	.threshold {
		stroke: var(--danger);
		stroke-width: 1.5;
		stroke-dasharray: 5 4;
	}

	.threshold-label {
		font-size: 11px;
		font-weight: 700;
		fill: var(--danger);
	}

	.cross {
		stroke: var(--border-strong);
		stroke-width: 1;
		stroke-dasharray: 3 3;
	}

	.empty {
		position: absolute;
		inset: 0 0 26px 56px;
		display: grid;
		place-items: center;
		text-align: center;
		padding: 1rem;
		font-size: 0.86rem;
		color: var(--muted);
		z-index: 1;
	}

	.tooltip {
		position: absolute;
		z-index: 2;
		pointer-events: none;
		min-width: 9.5rem;
		padding: 0.5rem 0.65rem;
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		box-shadow: var(--shadow-lg);
		font-size: 0.8rem;
		display: grid;
		gap: 0.2rem;
	}

	.tt-time {
		font-weight: 700;
		color: var(--muted);
		margin-bottom: 0.1rem;
	}

	.tt-row {
		display: flex;
		align-items: center;
		gap: 0.4rem;
	}

	.tt-label {
		flex: 1;
		color: var(--muted);
	}

	.tt-row strong {
		font-variant-numeric: tabular-nums;
	}

	.swatch {
		width: 0.65rem;
		height: 0.65rem;
		border-radius: 3px;
		flex-shrink: 0;
	}

	.legend {
		list-style: none;
		margin: 0.5rem 0 0;
		padding: 0 0 0 56px;
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem 1.1rem;
		font-size: 0.8rem;
		color: var(--muted);
	}

	.legend li {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
	}

	.legend strong {
		color: var(--text);
		font-variant-numeric: tabular-nums;
	}
</style>
