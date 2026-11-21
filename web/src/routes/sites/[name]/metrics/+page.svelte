<script lang="ts">
	import { onMount } from 'svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import StatCard from '#lib/components/StatCard.svelte';
	import TimeSeriesChart from '#lib/charts/TimeSeriesChart.svelte';
	import RangePicker from '#lib/charts/RangePicker.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { MetricsQuery, poll } from '#lib/stores/metrics.svelte.ts';
	import { formatBytes, formatPct, pctOf } from '#lib/format.ts';

	const site = getSite();
	const metrics = new MetricsQuery(site.path('/metrics'), '24h');

	let autoRefresh = $state(true);

	function stats(values: number[]) {
		const clean = values.filter((v) => v != null && !Number.isNaN(v));
		if (!clean.length) return { avg: 0, max: 0, last: 0, n: 0 };
		return {
			avg: clean.reduce((a, b) => a + b, 0) / clean.length,
			max: Math.max(...clean),
			last: clean[clean.length - 1],
			n: clean.length
		};
	}

	let cpu = $derived(stats(metrics.values('cpu_pct')));
	let cpuPeak = $derived(stats(metrics.values('cpu_max')));
	let mem = $derived(stats(metrics.values('mem_bytes')));
	let disk = $derived(stats(metrics.values('disk_bytes')));
	let uptimePct = $derived(stats(metrics.values('active')).avg * 100);

	const RANGE_LABEL = { '1h': 'last hour', '24h': 'last 24 hours', '7d': 'last 7 days' } as const;
	let rangeLabel = $derived(RANGE_LABEL[metrics.range]);

	onMount(() => {
		void metrics.load();
		return poll(() => {
			if (autoRefresh) void metrics.load();
		}, 60_000);
	});
</script>

<div class="toolbar">
	<RangePicker bind:value={metrics.range} onchange={() => metrics.load()} />
	<label class="auto">
		<input type="checkbox" bind:checked={autoRefresh} />
		Auto-refresh every minute
	</label>
	<button type="button" class="wbr-btn wbr-btn-ghost sm" disabled={metrics.loading} onclick={() => metrics.load()}>
		<Icon name="refresh" size={14} />
		{metrics.loading ? 'Loading…' : 'Refresh'}
	</button>
</div>

{#if metrics.error}<p class="wbr-error" role="alert">{metrics.error}</p>{/if}

<div class="stats">
	<StatCard label="CPU avg" icon="cpu" value={formatPct(cpu.avg)} sub={`peak ${formatPct(Math.max(cpu.max, cpuPeak.max))} · ${rangeLabel}`} />
	<StatCard
		label="Memory"
		icon="memory"
		value={formatBytes(mem.last)}
		sub={metrics.memLimit ? `peak ${formatBytes(mem.max)} · limit ${formatBytes(metrics.memLimit)}` : `peak ${formatBytes(mem.max)}`}
		pct={metrics.memLimit ? pctOf(mem.last, metrics.memLimit) : undefined}
	/>
	<StatCard
		label="Disk"
		icon="disk"
		value={formatBytes(disk.last)}
		sub={metrics.diskQuota ? `quota ${formatBytes(metrics.diskQuota)}` : 'no quota set'}
		pct={metrics.diskQuota ? pctOf(disk.last, metrics.diskQuota) : undefined}
	/>
	{#if site.isApp}
		<StatCard label="Running" icon="activity" value={formatPct(uptimePct, 0)} sub={`of samples, ${rangeLabel}`} />
	{/if}
</div>

{#if !metrics.data}
	<div class="charts">
		<Skeleton height="260px" radius="12px" />
		<Skeleton height="260px" radius="12px" />
	</div>
{:else}
	<div class="charts">
		<Card title="CPU" description="Share of total server CPU used by this site's unit">
			<TimeSeriesChart
				label={`${site.name} CPU usage, ${rangeLabel}`}
				t={metrics.data.t}
				step={metrics.data.step}
				from={metrics.data.from}
				to={metrics.data.to}
				kind="pct"
				series={[
					{ key: 'avg', label: 'Average', values: metrics.values('cpu_pct'), color: 'var(--chart-1)' },
					...(metrics.range === '1h'
						? []
						: [{ key: 'max', label: 'Peak', values: metrics.values('cpu_max'), color: 'var(--chart-4)', area: false, dashed: true }])
				]}
			/>
		</Card>
		<Card title="Memory" description="Resident memory of the app's systemd unit (cgroup)">
			<TimeSeriesChart
				label={`${site.name} memory usage, ${rangeLabel}`}
				t={metrics.data.t}
				step={metrics.data.step}
				from={metrics.data.from}
				to={metrics.data.to}
				kind="bytes"
				threshold={metrics.memLimit ? { value: metrics.memLimit, label: `limit ${formatBytes(metrics.memLimit)}` } : undefined}
				series={[{ key: 'mem', label: 'Memory', values: metrics.values('mem_bytes'), color: 'var(--chart-3)' }]}
				emptyText={site.isApp ? undefined : 'Static sites are served by Caddy and have no memory of their own.'}
			/>
		</Card>
		<Card title="Disk" description="Size of /srv/sites/{site.name} (measured every 15 minutes)">
			<TimeSeriesChart
				label={`${site.name} disk usage, ${rangeLabel}`}
				t={metrics.data.t}
				step={metrics.data.step}
				from={metrics.data.from}
				to={metrics.data.to}
				kind="bytes"
				threshold={metrics.diskQuota ? { value: metrics.diskQuota, label: `quota ${formatBytes(metrics.diskQuota)}` } : undefined}
				series={[{ key: 'disk', label: 'Disk', values: metrics.values('disk_bytes'), color: 'var(--chart-2)' }]}
			/>
		</Card>
	</div>
{/if}

<style>
	.toolbar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 1rem;
	}

	.auto {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.85rem;
		color: var(--muted);
		font-weight: 600;
		margin-right: auto;
	}

	.stats {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(11rem, 1fr));
		gap: 1rem;
		margin-bottom: 1rem;
	}

	.charts {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(min(26rem, 100%), 1fr));
		gap: 1rem;
	}
</style>
