<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import Card from '#lib/components/Card.svelte';
	import StatCard from '#lib/components/StatCard.svelte';
	import StatusPill from '#lib/components/StatusPill.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import ActivityFeed from '#lib/components/ActivityFeed.svelte';
	import AttentionList, { type Attention } from '#lib/components/AttentionList.svelte';
	import TimeSeriesChart from '#lib/charts/TimeSeriesChart.svelte';
	import Sparkline from '#lib/charts/Sparkline.svelte';
	import UsageBar from '#lib/charts/UsageBar.svelte';
	import StackedBar from '#lib/charts/StackedBar.svelte';
	import RangePicker from '#lib/charts/RangePicker.svelte';
	import { overview } from '#lib/stores/overview.svelte.ts';
	import { MetricsQuery, poll } from '#lib/stores/metrics.svelte.ts';
	import { ui } from '#lib/stores/ui.svelte.ts';
	import { request } from '#lib/api.ts';
	import { formatBytes, formatDuration, formatPct, formatRate, pctOf } from '#lib/format.ts';
	import { siteStatus } from '#lib/siteStatus.ts';
	import type { AuditEvent, OverviewSite } from '#lib/types.ts';

	const host = new MetricsQuery('/api/metrics/host', '1h');

	let events = $state.raw<AuditEvent[]>([]);
	let eventsLoaded = $state(false);
	let topBy = $state<'cpu' | 'mem' | 'disk'>('mem');
	let shareBy = $state<'mem' | 'disk'>('mem');

	let data = $derived(overview.data);
	let h = $derived(data?.host);
	let sites = $derived(overview.sites);
	let loading = $derived(!data && overview.loading);

	const PALETTE = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)'];

	const siteHref = (s: OverviewSite) => `/sites/${encodeURIComponent(s.name)}`;

	let runningApps = $derived(sites.filter((s) => s.type === 'app' && s.active).length);
	let apps = $derived(sites.filter((s) => s.type === 'app').length);
	let httpsCount = $derived(sites.filter((s) => s.https).length);

	let spark = $derived(data?.host_spark?.series);

	let topSites = $derived.by(() => {
		const metric = (s: OverviewSite) =>
			topBy === 'cpu' ? s.cpu_pct : topBy === 'mem' ? s.mem_bytes : s.disk_bytes;
		return [...sites].sort((a, b) => metric(b) - metric(a)).slice(0, 6);
	});

	let topMax = $derived(
		Math.max(
			...topSites.map((s) => (topBy === 'cpu' ? s.cpu_pct : topBy === 'mem' ? s.mem_bytes : s.disk_bytes)),
			0
		)
	);

	let shareSegments = $derived.by(() => {
		const key = shareBy === 'mem' ? 'mem_bytes' : 'disk_bytes';
		const ranked = [...sites].filter((s) => s[key] > 0).sort((a, b) => b[key] - a[key]);
		const top = ranked.slice(0, 5).map((s, i) => ({
			key: s.name,
			label: s.name,
			value: s[key],
			color: PALETTE[i % PALETTE.length],
			href: siteHref(s)
		}));
		const restSites = ranked.slice(5).reduce((a, s) => a + s[key], 0);
		if (restSites > 0) {
			top.push({ key: '__rest', label: `${ranked.length - 5} more sites`, value: restSites, color: 'var(--subtle)', href: '/sites' });
		}
		return top;
	});

	let attention = $derived.by((): Attention[] => {
		if (!data || !h) return [];
		const out: Attention[] = [];
		const diskPct = pctOf(h.disk_used, h.disk_total);
		const memPct = pctOf(h.mem_used, h.mem_total);
		if (diskPct >= 85) {
			out.push({
				key: 'disk',
				tone: diskPct >= 95 ? 'danger' : 'warn',
				title: `Server disk is ${Math.round(diskPct)}% full`,
				detail: `${formatBytes(h.disk_total - h.disk_used)} free. Clear old releases or logs.`
			});
		}
		if (memPct >= 90) {
			out.push({
				key: 'mem',
				tone: 'warn',
				title: `Memory is ${Math.round(memPct)}% used`,
				detail: 'Apps may be killed by the OOM killer if usage keeps growing.'
			});
		}
		for (const s of sites) {
			if (s.mem_limit_bytes && s.mem_bytes / s.mem_limit_bytes >= 0.8) {
				out.push({
					key: `mem:${s.name}`,
					tone: s.mem_bytes / s.mem_limit_bytes >= 0.95 ? 'danger' : 'warn',
					title: `${s.name} is near its memory limit`,
					detail: `${formatBytes(s.mem_bytes)} of ${s.memory_max}`,
					href: `${siteHref(s)}/metrics`,
					action: 'Metrics'
				});
			}
			if (s.disk_quota_bytes && s.disk_bytes / s.disk_quota_bytes >= 0.8) {
				out.push({
					key: `disk:${s.name}`,
					tone: 'warn',
					title: `${s.name} is near its disk quota`,
					detail: `${formatBytes(s.disk_bytes)} of ${s.disk_quota}`,
					href: `${siteHref(s)}/metrics`,
					action: 'Metrics'
				});
			}
		}
		if (data.pending_approvals > 0) {
			out.push({
				key: 'approvals',
				tone: 'warn',
				title: `${data.pending_approvals} approval${data.pending_approvals === 1 ? '' : 's'} waiting`,
				detail: 'Run sudo wbr approve <id> on the server to continue the action.'
			});
		}
		const noHttps = sites.filter((s) => !s.https);
		if (noHttps.length && !data.https_disabled) {
			out.push({
				key: 'https',
				tone: 'info',
				title: `${noHttps.length} site${noHttps.length === 1 ? '' : 's'} without HTTPS`,
				detail: noHttps.slice(0, 3).map((s) => s.name).join(', ') + (noHttps.length > 3 ? '…' : ''),
				href: `/sites/${encodeURIComponent(noHttps[0].name)}/domains`,
				action: 'Set up'
			});
		}
		if (!data.sampled) {
			out.push({
				key: 'sampling',
				tone: 'info',
				title: 'Collecting the first resource samples',
				detail: 'CPU and network figures appear within a few seconds; graphs fill in over time.'
			});
		}
		return out;
	});

	async function loadEvents() {
		try {
			const r = await request<{ events?: AuditEvent[] }>('/api/audit?limit=8');
			if (r.ok) events = r.data.events ?? [];
		} catch {
			// activity is optional on the dashboard
		} finally {
			eventsLoaded = true;
		}
	}

	function refreshAll() {
		void overview.refresh();
		void host.load();
		void loadEvents();
	}

	onMount(() => {
		void overview.ensure(5_000);
		void host.load();
		void loadEvents();
		const stopOverview = poll(() => void overview.refresh(), 30_000);
		const stopHost = poll(() => void host.load(), 60_000);
		return () => {
			stopOverview();
			stopHost();
		};
	});
</script>

<svelte:head>
	<title>Dashboard · Webor</title>
</svelte:head>

<PageHeader
	title="Dashboard"
	subtitle={h
		? `${h.hostname || 'Server'} · ${h.cpu_count} vCPU · ${formatBytes(h.mem_total)} RAM · up ${formatDuration(h.uptime_sec)}`
		: 'Server health and site resource usage at a glance'}
>
	{#snippet actions()}
		<button type="button" class="wbr-btn wbr-btn-ghost" onclick={refreshAll} disabled={overview.loading}>
			<Icon name="refresh" size={15} /> Refresh
		</button>
		<button type="button" class="wbr-btn" onclick={() => (ui.addSiteOpen = true)}>
			<Icon name="plus" size={15} /> Add site
		</button>
	{/snippet}
</PageHeader>

{#if overview.error && !data}
	<p class="wbr-error" role="alert">{overview.error}</p>
{/if}

{#if attention.length}
	<div class="block"><AttentionList items={attention} /></div>
{/if}

<div class="stats">
	<StatCard
		label="CPU"
		icon="cpu"
		{loading}
		value={h ? formatPct(h.cpu_pct) : '—'}
		sub={h ? `Load ${h.load1.toFixed(2)} · ${h.load5.toFixed(2)} · ${h.load15.toFixed(2)}` : undefined}
		pct={h?.cpu_pct}
	>
		<Sparkline values={spark?.cpu_pct ?? []} max={100} color="var(--chart-1)" />
	</StatCard>
	<StatCard
		label="Memory"
		icon="memory"
		{loading}
		value={h ? formatBytes(h.mem_used) : '—'}
		sub={h ? `of ${formatBytes(h.mem_total)} · ${formatPct(pctOf(h.mem_used, h.mem_total))}` : undefined}
		pct={h ? pctOf(h.mem_used, h.mem_total) : undefined}
	>
		<Sparkline values={spark?.mem_used ?? []} max={h?.mem_total} color="var(--chart-3)" />
	</StatCard>
	<StatCard
		label="Disk"
		icon="disk"
		{loading}
		value={h ? formatBytes(h.disk_used) : '—'}
		sub={h ? `of ${formatBytes(h.disk_total)} · ${formatBytes(h.disk_total - h.disk_used)} free` : undefined}
		pct={h ? pctOf(h.disk_used, h.disk_total) : undefined}
	/>
	<StatCard
		label="Network"
		icon="network"
		{loading}
		value={h ? formatRate(h.net_rx + h.net_tx) : '—'}
		sub={h ? `↓ ${formatRate(h.net_rx)} · ↑ ${formatRate(h.net_tx)}` : undefined}
	>
		<Sparkline values={spark?.net_rx ?? []} color="var(--chart-2)" />
	</StatCard>
	<StatCard
		label="Sites"
		icon="globe"
		{loading}
		value={String(sites.length)}
		sub={`${runningApps}/${apps} apps running · ${httpsCount} with HTTPS`}
	/>
</div>

<div class="grid-2">
	<Card title="CPU usage" description="Share of all cores used by the whole server">
		{#snippet actions()}
			<RangePicker bind:value={host.range} onchange={() => host.load()} />
		{/snippet}
		{#if host.error}<p class="wbr-error">{host.error}</p>{/if}
		{#if host.data}
			<TimeSeriesChart
				label="Server CPU usage"
				t={host.data.t}
				step={host.data.step}
				from={host.data.from}
				to={host.data.to}
				kind="pct"
				yMax={100}
				series={[
					{ key: 'avg', label: 'Average', values: host.values('cpu_pct'), color: 'var(--chart-1)' },
					...(host.range === '1h'
						? []
						: [{ key: 'max', label: 'Peak', values: host.values('cpu_max'), color: 'var(--chart-4)', area: false, dashed: true }])
				]}
			/>
		{:else}
			<Skeleton height="220px" radius="10px" />
		{/if}
	</Card>

	<Card title="Memory usage" description="Used memory (excluding reclaimable cache)">
		{#snippet actions()}
			<RangePicker bind:value={host.range} onchange={() => host.load()} />
		{/snippet}
		{#if host.data}
			<TimeSeriesChart
				label="Server memory usage"
				t={host.data.t}
				step={host.data.step}
				from={host.data.from}
				to={host.data.to}
				kind="bytes"
				yMax={h?.mem_total || undefined}
				series={[{ key: 'mem', label: 'Used', values: host.values('mem_used'), color: 'var(--chart-3)' }]}
			/>
		{:else}
			<Skeleton height="220px" radius="10px" />
		{/if}
	</Card>
</div>

<div class="grid-2">
	<Card title="Top sites" description="Heaviest sites right now">
		{#snippet actions()}
			<div class="seg" role="radiogroup" aria-label="Rank by">
				{#each [['mem', 'Memory'], ['cpu', 'CPU'], ['disk', 'Disk']] as [k, l] (k)}
					<button
						type="button"
						role="radio"
						aria-checked={topBy === k}
						class={[topBy === k && 'on']}
						onclick={() => (topBy = k as typeof topBy)}>{l}</button
					>
				{/each}
			</div>
		{/snippet}
		{#if loading}
			<Skeleton lines={4} height="1.4rem" />
		{:else if sites.length === 0}
			<EmptyState icon="globe" title="No sites yet" description="Add a site to see its resource usage here." compact />
		{:else}
			<ol class="ranking">
				{#each topSites as s (s.name)}
					{@const limit = topBy === 'mem' ? s.mem_limit_bytes : topBy === 'disk' ? s.disk_quota_bytes : 100}
					<li>
						<a href={`${siteHref(s)}/metrics`} class="rank-name">{s.name}</a>
						<UsageBar
							compact
							used={topBy === 'cpu' ? s.cpu_pct : topBy === 'mem' ? s.mem_bytes : s.disk_bytes}
							total={limit || topMax || 1}
							format={topBy === 'cpu' ? (v) => formatPct(v) : (v) => formatBytes(v)}
						/>
						<span class="rank-val">
							{topBy === 'cpu' ? formatPct(s.cpu_pct) : formatBytes(topBy === 'mem' ? s.mem_bytes : s.disk_bytes)}
							{#if topBy === 'mem' && s.memory_max}<small>/ {s.memory_max}</small>{/if}
							{#if topBy === 'disk' && s.disk_quota}<small>/ {s.disk_quota}</small>{/if}
						</span>
					</li>
				{/each}
			</ol>
		{/if}
	</Card>

	<Card title="Resource share" description="How the server's capacity is split between sites">
		{#snippet actions()}
			<div class="seg" role="radiogroup" aria-label="Resource">
				{#each [['mem', 'Memory'], ['disk', 'Disk']] as [k, l] (k)}
					<button
						type="button"
						role="radio"
						aria-checked={shareBy === k}
						class={[shareBy === k && 'on']}
						onclick={() => (shareBy = k as typeof shareBy)}>{l}</button
					>
				{/each}
			</div>
		{/snippet}
		{#if loading || !h}
			<Skeleton lines={4} height="1.2rem" />
		{:else}
			<StackedBar
				label={shareBy === 'mem' ? 'Memory by site' : 'Disk by site'}
				segments={shareSegments}
				total={shareBy === 'mem' ? h.mem_used : h.disk_used}
				restLabel={shareBy === 'mem' ? 'System & other processes' : 'System & other files'}
				format={(v) => formatBytes(v)}
			/>
		{/if}
	</Card>
</div>

<div class="grid-main">
	<Card title="Sites" flush>
		{#snippet actions()}
			<a class="wbr-btn wbr-btn-ghost sm" href="/sites">View all <Icon name="chevronRight" size={14} /></a>
		{/snippet}
		{#if loading}
			<div class="pad"><Skeleton lines={5} height="1.6rem" /></div>
		{:else if sites.length === 0}
			<EmptyState icon="globe" title="No sites yet" description="Create your first site to start hosting.">
				<button type="button" class="wbr-btn" onclick={() => (ui.addSiteOpen = true)}>
					<Icon name="plus" size={15} /> Add site
				</button>
			</EmptyState>
		{:else}
			<table class="wbr-table">
				<thead>
					<tr>
						<th>Site</th>
						<th>Status</th>
						<th class="num">CPU</th>
						<th class="num">Memory</th>
						<th class="num hide-sm">Disk</th>
						<th class="hide-sm">Last hour</th>
					</tr>
				</thead>
				<tbody>
					{#each sites.slice(0, 8) as s (s.name)}
						{@const st = siteStatus(s)}
						<tr>
							<td>
								<a class="site" href={siteHref(s)}>{s.name}</a>
								<div class="tags">
									<span class="wbr-badge neutral">{s.type}</span>
									{#if s.https}<span class="wbr-badge ok"><Icon name="lock" size={11} /> HTTPS</span>{/if}
								</div>
							</td>
							<td><StatusPill tone={st.tone} label={st.label} /></td>
							<td class="num">{formatPct(s.cpu_pct)}</td>
							<td class="num">{formatBytes(s.mem_bytes)}</td>
							<td class="num hide-sm">{formatBytes(s.disk_bytes)}</td>
							<td class="spark hide-sm">
								<Sparkline values={s.spark?.series.mem_bytes ?? []} color="var(--chart-3)" height={26} />
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</Card>

	<Card title="Recent activity">
		{#snippet actions()}
			<a class="wbr-btn wbr-btn-ghost sm" href="/activity">All <Icon name="chevronRight" size={14} /></a>
		{/snippet}
		{#if !eventsLoaded}
			<Skeleton lines={4} height="1.4rem" />
		{:else if events.length === 0}
			<EmptyState icon="activity" title="No activity yet" description="Privileged actions are logged here." compact />
		{:else}
			<ActivityFeed {events} />
		{/if}
	</Card>
</div>

<style>
	.block {
		margin-bottom: 1rem;
	}

	.stats {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(11.5rem, 1fr));
		gap: 1rem;
		margin-bottom: 1rem;
	}

	.grid-2 {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(min(24rem, 100%), 1fr));
		gap: 1rem;
		margin-bottom: 1rem;
	}

	.grid-main {
		display: grid;
		grid-template-columns: minmax(0, 2fr) minmax(16rem, 1fr);
		gap: 1rem;
		align-items: start;
	}

	.seg {
		display: inline-flex;
		padding: 2px;
		border-radius: var(--radius-sm);
		background: var(--surface-3);
		border: 1px solid var(--border);
	}

	.seg button {
		font: inherit;
		font-size: 0.76rem;
		font-weight: 800;
		padding: 0.25rem 0.55rem;
		border: 0;
		border-radius: 5px;
		background: none;
		color: var(--muted);
		cursor: pointer;
	}

	.seg button.on {
		background: var(--surface);
		color: var(--accent);
		box-shadow: var(--shadow-sm);
	}

	.ranking {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.7rem;
	}

	.ranking li {
		display: grid;
		grid-template-columns: minmax(6rem, 11rem) 1fr auto;
		gap: 0.75rem;
		align-items: center;
	}

	.rank-name {
		font-weight: 700;
		font-size: 0.88rem;
		color: var(--text);
		text-decoration: none;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.rank-name:hover {
		color: var(--accent);
	}

	.rank-val {
		font-size: 0.85rem;
		font-weight: 700;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
		min-width: 5rem;
		text-align: right;
	}

	.rank-val small {
		font-weight: 600;
		color: var(--subtle);
	}

	.pad {
		padding: 0 1.25rem 1.25rem;
	}

	.site {
		font-weight: 800;
		color: var(--text);
		text-decoration: none;
		overflow-wrap: anywhere;
	}

	.site:hover {
		color: var(--accent);
	}

	.tags {
		display: flex;
		gap: 0.3rem;
		margin-top: 0.2rem;
	}

	.num {
		text-align: right;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}

	.spark {
		width: 8rem;
	}

	@media (max-width: 1100px) {
		.grid-main {
			grid-template-columns: 1fr;
		}
	}

	@media (max-width: 640px) {
		.hide-sm {
			display: none;
		}

		.ranking li {
			grid-template-columns: 1fr auto;
		}

		.ranking li :global(.usage) {
			grid-column: 1 / -1;
			grid-row: 2;
		}
	}
</style>
