<script lang="ts">
	import { onMount } from 'svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import Gauge from '#lib/charts/Gauge.svelte';
	import TimeSeriesChart from '#lib/charts/TimeSeriesChart.svelte';
	import type { IconName } from '#lib/icons/paths.ts';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { MetricsQuery, poll } from '#lib/stores/metrics.svelte.ts';
	import { overview } from '#lib/stores/overview.svelte.ts';
	import { request } from '#lib/api.ts';
	import { formatBytes, formatPct, pctOf } from '#lib/format.ts';

	const site = getSite();

	type Resources = {
		disk_used_bytes?: number;
		disk_avail_bytes?: number;
		disk_quota?: string;
		memory_current_bytes?: number;
		memory_max?: string;
		cpu_usage_usec?: number;
		app_active?: boolean;
		unit?: string;
	};

	let resources = $state.raw<Resources | null>(null);
	let resourcesError = $state('');
	let resourcesLoading = $state(false);
	let dbCount = $state<number | null>(null);
	let cacheAttached = $state<boolean | null>(null);

	const metrics = new MetricsQuery(site.path('/metrics'), '24h');

	let summary = $derived(overview.sites.find((s) => s.name.toLowerCase() === site.name.toLowerCase()));
	let memLimit = $derived(summary?.mem_limit_bytes || metrics.memLimit || 0);
	let diskQuota = $derived(summary?.disk_quota_bytes || metrics.diskQuota || 0);

	let memUsed = $derived(resources?.memory_current_bytes ?? summary?.mem_bytes ?? 0);
	let diskUsed = $derived(resources?.disk_used_bytes ?? summary?.disk_bytes ?? 0);

	type Step = { key: string; label: string; done: boolean | null; href: string; icon: IconName; hint: string };

	let steps = $derived.by((): Step[] => [
		site.isApp
			? {
					key: 'deploy',
					label: 'Deploy a release',
					done: resources ? !!resources.app_active || !!summary?.active : null,
					href: site.href('/deploy'),
					icon: 'rocket',
					hint: 'Upload a binary or build from git'
				}
			: {
					key: 'upload',
					label: 'Upload site files',
					done: null,
					href: site.href('/files'),
					icon: 'upload',
					hint: `Files go under ${site.root}/`
				},
		{
			key: 'dns',
			label: 'Point DNS at this server',
			done: site.https ? true : null,
			href: site.href('/domains'),
			icon: 'globe',
			hint: 'Run the DNS check'
		},
		{
			key: 'https',
			label: 'Enable HTTPS',
			done: site.https,
			href: site.href('/domains'),
			icon: 'lock',
			hint: "Free certificate from Let's Encrypt"
		},
		{
			key: 'db',
			label: 'Attach a database',
			done: dbCount == null ? null : dbCount > 0,
			href: site.href('/databases'),
			icon: 'database',
			hint: 'SQLite, Postgres or MariaDB (optional)'
		},
		{
			key: 'cache',
			label: 'Attach a cache',
			done: cacheAttached,
			href: site.href('/cache'),
			icon: 'zap',
			hint: 'Install Valkey as root, then attach (optional)'
		}
	]);

	let doneCount = $derived(steps.filter((s) => s.done).length);

	async function loadResources() {
		resourcesLoading = true;
		try {
			const r = await request<{ resources?: Resources }>(site.path('/resources'));
			if (r.ok) {
				resources = r.data.resources ?? null;
				resourcesError = '';
			} else {
				resourcesError = r.data.error || 'Could not load resources';
			}
		} catch (e) {
			resourcesError = e instanceof Error ? e.message : 'Could not load resources';
		} finally {
			resourcesLoading = false;
		}
	}

	async function loadExtras() {
		try {
			const [db, cache] = await Promise.all([
				request<{ databases?: unknown[] }>(site.path('/databases')),
				request<{ attached?: boolean; username?: string; cache?: { attached?: boolean; username?: string } | null }>(
					site.path('/cache')
				)
			]);
			if (db.ok) dbCount = db.data.databases?.length ?? 0;
			if (cache.ok) {
				const c = cache.data.cache;
				cacheAttached = !!(c ? (c.attached ?? c.username) : (cache.data.attached ?? cache.data.username));
			}
		} catch {
			// checklist falls back to "unknown"
		}
	}

	onMount(() => {
		void loadResources();
		void loadExtras();
		void metrics.load();
		void overview.ensure(30_000);
		return poll(() => {
			void metrics.load();
			void overview.refresh();
		}, 60_000);
	});
</script>

<div class="layout">
	<div class="main">
		<Card title="Resources" description="Live usage compared with this site's limits">
			{#snippet actions()}
				<button
					type="button"
					class="wbr-btn wbr-btn-ghost sm"
					disabled={resourcesLoading}
					onclick={loadResources}
				>
					<Icon name="refresh" size={14} />
					{resourcesLoading ? 'Refreshing…' : 'Refresh'}
				</button>
			{/snippet}
			{#if resourcesError}<p class="wbr-error">{resourcesError}</p>{/if}
			{#if !resources && resourcesLoading}
				<Skeleton height="8rem" radius="10px" />
			{:else}
				<div class="gauges">
					<Gauge
						label="CPU"
						value={formatPct(summary?.cpu_pct ?? 0)}
						pct={summary?.cpu_pct ?? 0}
						sub="of server capacity"
					/>
					<Gauge
						label="Memory"
						value={formatBytes(memUsed)}
						pct={memLimit ? pctOf(memUsed, memLimit) : undefined}
						sub={memLimit ? `limit ${resources?.memory_max || summary?.memory_max}` : site.isApp ? 'no limit set' : 'served by Caddy'}
					/>
					<Gauge
						label="Disk"
						value={formatBytes(diskUsed)}
						pct={diskQuota ? pctOf(diskUsed, diskQuota) : undefined}
						sub={diskQuota
							? `quota ${resources?.disk_quota || summary?.disk_quota}`
							: resources?.disk_avail_bytes
								? `${formatBytes(resources.disk_avail_bytes)} free on volume`
								: 'no quota set'}
					/>
				</div>
			{/if}
		</Card>

		<Card title="Last 24 hours">
			{#snippet actions()}
				<a class="wbr-btn wbr-btn-ghost sm" href={site.href('/metrics')}>
					All metrics <Icon name="chevronRight" size={14} />
				</a>
			{/snippet}
			{#if metrics.data}
				<div class="minis">
					<div>
						<h3>CPU</h3>
						<TimeSeriesChart
							label={`${site.name} CPU, last 24 hours`}
							t={metrics.data.t}
							step={metrics.data.step}
							from={metrics.data.from}
							to={metrics.data.to}
							kind="pct"
							height={150}
							series={[{ key: 'cpu', label: 'CPU', values: metrics.values('cpu_pct'), color: 'var(--chart-1)' }]}
						/>
					</div>
					<div>
						<h3>Memory</h3>
						<TimeSeriesChart
							label={`${site.name} memory, last 24 hours`}
							t={metrics.data.t}
							step={metrics.data.step}
							from={metrics.data.from}
							to={metrics.data.to}
							kind="bytes"
							height={150}
							threshold={memLimit ? { value: memLimit, label: 'limit' } : undefined}
							series={[{ key: 'mem', label: 'Memory', values: metrics.values('mem_bytes'), color: 'var(--chart-3)' }]}
						/>
					</div>
				</div>
			{:else if metrics.error}
				<p class="wbr-error">{metrics.error}</p>
			{:else}
				<Skeleton height="150px" radius="10px" />
			{/if}
		</Card>
	</div>

	<aside class="side">
		<Card title="Setup checklist" description={`${doneCount} of ${steps.length} done`}>
			<ol class="steps">
				{#each steps as s (s.key)}
					<li class={[s.done === true && 'done']}>
						<span class="check" aria-hidden="true">
							{#if s.done}<Icon name="check" size={13} />{:else}<Icon name={s.icon} size={13} />{/if}
						</span>
						<a href={s.href}>
							<strong>{s.label}</strong>
							<span>{s.done === true ? 'Done' : s.hint}</span>
						</a>
						<span class="visually-hidden">
							{s.done === true ? 'complete' : s.done === false ? 'not done' : 'status unknown'}
						</span>
					</li>
				{/each}
			</ol>
		</Card>

		<Card title="Details">
			<ul class="wbr-facts">
				<li><span>Type</span> <strong>{site.type}</strong></li>
				<li><span>Home</span> <code>/srv/sites/{site.name}/</code></li>
				{#if !site.isApp}<li><span>Web root</span> <code>{site.root}/</code></li>{/if}
				{#if resources?.unit}<li><span>Unit</span> <code>{resources.unit}</code></li>{/if}
				{#if site.isChild}
					<li>
						<span>Parent</span>
						<a href={`/sites/${encodeURIComponent(site.parent)}`}>{site.parent}</a>
					</li>
				{/if}
				{#if site.socketProxy}<li><span>Socket</span> <strong>proxyd (idle-stop)</strong></li>{/if}
				{#if site.cron.length}<li><span>Cron</span> <span class="list">{site.cron.join(', ')}</span></li>{/if}
				{#if site.jobs.length}<li><span>Jobs</span> <span class="list">{site.jobs.join(', ')}</span></li>{/if}
			</ul>
		</Card>
	</aside>
</div>

<style>
	.layout {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(17rem, 22rem);
		gap: 1rem;
		align-items: start;
	}

	.main,
	.side {
		display: grid;
		gap: 1rem;
		min-width: 0;
	}

	.gauges {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
		gap: 1rem;
		padding: 0.25rem 0;
	}

	.minis {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(min(18rem, 100%), 1fr));
		gap: 1rem;
	}

	h3 {
		margin: 0 0 0.4rem;
		font-size: 0.78rem;
		font-weight: 800;
		color: var(--subtle);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.steps {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.15rem;
	}

	.steps li {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 0.65rem;
		align-items: center;
	}

	.check {
		display: grid;
		place-items: center;
		width: 1.6rem;
		height: 1.6rem;
		border-radius: 50%;
		border: 1.5px solid var(--border-strong);
		color: var(--subtle);
	}

	.done .check {
		background: var(--ok);
		border-color: var(--ok);
		color: #fff;
	}

	.steps a {
		display: grid;
		padding: 0.4rem 0.5rem;
		border-radius: var(--radius-sm);
		text-decoration: none;
		color: var(--text);
	}

	.steps a:hover {
		background: var(--surface-3);
	}

	.steps strong {
		font-size: 0.88rem;
	}

	.done strong {
		color: var(--muted);
		text-decoration: line-through;
		text-decoration-color: var(--border-strong);
	}

	.steps a span {
		font-size: 0.78rem;
		color: var(--subtle);
	}

	.list {
		font-size: 0.88rem;
		overflow-wrap: anywhere;
	}

	@media (max-width: 1100px) {
		.layout {
			grid-template-columns: 1fr;
		}
	}
</style>
