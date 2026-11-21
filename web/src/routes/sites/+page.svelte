<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import Card from '#lib/components/Card.svelte';
	import StatusPill from '#lib/components/StatusPill.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Sparkline from '#lib/charts/Sparkline.svelte';
	import UsageBar from '#lib/charts/UsageBar.svelte';
	import { overview } from '#lib/stores/overview.svelte.ts';
	import { poll } from '#lib/stores/metrics.svelte.ts';
	import { ui } from '#lib/stores/ui.svelte.ts';
	import { formatBytes, formatPct } from '#lib/format.ts';
	import { siteStatus } from '#lib/siteStatus.ts';
	import type { OverviewSite } from '#lib/types.ts';

	type SortKey = 'name' | 'cpu' | 'mem' | 'disk';

	let query = $state('');
	let typeFilter = $state<'all' | 'static' | 'app'>('all');
	let statusFilter = $state<'all' | 'running' | 'idle' | 'nohttps'>('all');
	let sortKey = $state<SortKey>('name');
	let sortDesc = $state(false);

	let sites = $derived(overview.sites);
	let loading = $derived(!overview.data && overview.loading);

	let rows = $derived.by(() => {
		const q = query.trim().toLowerCase();
		const filtered = sites.filter((s) => {
			if (q && !s.name.toLowerCase().includes(q) && !(s.parent ?? '').toLowerCase().includes(q)) return false;
			if (typeFilter !== 'all' && s.type !== typeFilter) return false;
			if (statusFilter === 'running' && !(s.type !== 'app' || s.active)) return false;
			if (statusFilter === 'idle' && !(s.type === 'app' && !s.active)) return false;
			if (statusFilter === 'nohttps' && s.https) return false;
			return true;
		});
		const val = (s: OverviewSite): number | string =>
			sortKey === 'name' ? s.name : sortKey === 'cpu' ? s.cpu_pct : sortKey === 'mem' ? s.mem_bytes : s.disk_bytes;
		return filtered.sort((a, b) => {
			const va = val(a);
			const vb = val(b);
			const cmp = typeof va === 'string' ? va.localeCompare(vb as string) : va - (vb as number);
			return sortDesc ? -cmp : cmp;
		});
	});

	let filtersActive = $derived(!!query || typeFilter !== 'all' || statusFilter !== 'all');

	function setSort(k: SortKey) {
		if (sortKey === k) sortDesc = !sortDesc;
		else {
			sortKey = k;
			sortDesc = k !== 'name';
		}
	}

	function ariaSort(k: SortKey) {
		if (sortKey !== k) return 'none';
		return sortDesc ? 'descending' : 'ascending';
	}

	function clearFilters() {
		query = '';
		typeFilter = 'all';
		statusFilter = 'all';
	}

	const siteHref = (s: OverviewSite, rest = '') => `/sites/${encodeURIComponent(s.name)}${rest}`;

	onMount(() => {
		void overview.refresh();
		return poll(() => void overview.refresh(), 30_000);
	});
</script>

<svelte:head>
	<title>Sites · Webor</title>
</svelte:head>

{#snippet sortHead(k: SortKey, label: string, cls = '')}
	<th class={cls} aria-sort={ariaSort(k)}>
		<button type="button" class="sort" onclick={() => setSort(k)}>
			{label}
			{#if sortKey === k}
				<Icon name="chevronDown" size={13} class={sortDesc ? '' : 'flip'} />
			{/if}
		</button>
	</th>
{/snippet}

<PageHeader title="Sites" subtitle="Static and app sites hosted on this server">
	{#snippet meta()}
		{#if sites.length}<span class="wbr-badge neutral">{sites.length}</span>{/if}
	{/snippet}
	{#snippet actions()}
		<button type="button" class="wbr-btn" onclick={() => (ui.addSiteOpen = true)}>
			<Icon name="plus" size={15} /> Add site
		</button>
	{/snippet}
</PageHeader>

{#if overview.error && !overview.data}
	<p class="wbr-error" role="alert">{overview.error}</p>
{/if}

<Card flush>
	<div class="toolbar">
		<label class="search">
			<Icon name="search" size={15} />
			<span class="visually-hidden">Search sites</span>
			<input bind:value={query} placeholder="Search by domain…" autocomplete="off" spellcheck="false" />
		</label>
		<label class="select">
			<span class="visually-hidden">Type</span>
			<select class="wbr-input" bind:value={typeFilter}>
				<option value="all">All types</option>
				<option value="static">Static</option>
				<option value="app">App</option>
			</select>
		</label>
		<label class="select">
			<span class="visually-hidden">Status</span>
			<select class="wbr-input" bind:value={statusFilter}>
				<option value="all">Any status</option>
				<option value="running">Serving / running</option>
				<option value="idle">Idle apps</option>
				<option value="nohttps">Without HTTPS</option>
			</select>
		</label>
		{#if filtersActive}
			<button type="button" class="wbr-btn wbr-btn-ghost sm" onclick={clearFilters}>Clear</button>
		{/if}
		<span class="result-count">{rows.length} of {sites.length}</span>
	</div>

	{#if loading}
		<div class="pad"><Skeleton lines={6} height="2rem" /></div>
	{:else if sites.length === 0}
		<EmptyState
			icon="globe"
			title="No sites yet"
			description="Add a domain to create its site user, directories and Caddy route."
		>
			<button type="button" class="wbr-btn" onclick={() => (ui.addSiteOpen = true)}>
				<Icon name="plus" size={15} /> Add your first site
			</button>
		</EmptyState>
	{:else if rows.length === 0}
		<EmptyState icon="search" title="No sites match" description="Try a different search or clear the filters.">
			<button type="button" class="wbr-btn wbr-btn-ghost" onclick={clearFilters}>Clear filters</button>
		</EmptyState>
	{:else}
		<div class="table-wrap">
			<table class="wbr-table">
				<thead>
					<tr>
						{@render sortHead('name', 'Site')}
						<th>Status</th>
						{@render sortHead('cpu', 'CPU', 'num')}
						{@render sortHead('mem', 'Memory', 'mem-col')}
						{@render sortHead('disk', 'Disk', 'num hide-md')}
						<th class="hide-md">CPU · 1h</th>
						<th><span class="visually-hidden">Actions</span></th>
					</tr>
				</thead>
				<tbody>
					{#each rows as s (s.name)}
						{@const st = siteStatus(s)}
						<tr>
							<td>
								<a class="site" href={siteHref(s)}>{s.name}</a>
								<div class="tags">
									<span class="wbr-badge neutral">{s.type}</span>
									{#if s.https}
										<span class="wbr-badge ok"><Icon name="lock" size={11} /> HTTPS</span>
									{:else}
										<span class="wbr-badge warn">HTTP only</span>
									{/if}
									{#if s.parent}
										<span class="wbr-badge neutral" title={`Subdomain of ${s.parent}`}>sub of {s.parent}</span>
									{:else if (s.subdomain_count ?? 0) > 0}
										<span class="wbr-badge neutral">{s.subdomain_count} sub</span>
									{/if}
									{#if s.database_count}<span class="wbr-badge neutral"><Icon name="database" size={11} /> {s.database_count}</span>{/if}
									{#if s.cache}<span class="wbr-badge neutral"><Icon name="zap" size={11} /> cache</span>{/if}
								</div>
							</td>
							<td><StatusPill tone={st.tone} label={st.label} /></td>
							<td class="num">{formatPct(s.cpu_pct)}</td>
							<td class="mem-col">
								<span class="mem-val">
									{formatBytes(s.mem_bytes)}
									{#if s.memory_max}<small class="quota">/ {s.memory_max}</small>{/if}
								</span>
								{#if s.mem_limit_bytes}
									<UsageBar compact used={s.mem_bytes} total={s.mem_limit_bytes} format={(v) => formatBytes(v)} />
								{/if}
							</td>
							<td class="num hide-md">
								{formatBytes(s.disk_bytes)}
								{#if s.disk_quota}<small class="quota">/ {s.disk_quota}</small>{/if}
							</td>
							<td class="spark hide-md">
								<Sparkline values={s.spark?.series.cpu_pct ?? []} height={26} label={`${s.name} CPU last hour`} />
							</td>
							<td class="actions">
								<a class="wbr-btn wbr-btn-ghost sm" href={siteHref(s)}>Open</a>
								<a class="icon-link" href={siteHref(s, '/settings')} aria-label={`${s.name} settings`} title="Settings">
									<Icon name="settings" size={15} />
								</a>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</Card>

<style>
	.toolbar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.6rem;
		padding: 0.25rem 1.25rem 0.9rem;
		border-bottom: 1px solid var(--border);
	}

	.search {
		flex: 1 1 16rem;
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.45rem 0.7rem;
		border: 1px solid var(--border-strong);
		border-radius: var(--radius-sm);
		background: var(--surface);
		color: var(--subtle);
	}

	.search:focus-within {
		border-color: var(--accent);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 22%, transparent);
	}

	.search input {
		flex: 1;
		min-width: 0;
		border: 0;
		outline: none;
		background: none;
		font: inherit;
		font-size: 0.9rem;
		color: var(--text);
	}

	.select select {
		padding-block: 0.45rem;
	}

	.result-count {
		margin-left: auto;
		font-size: 0.8rem;
		color: var(--subtle);
		white-space: nowrap;
	}

	.pad {
		padding: 1rem 1.25rem;
	}

	.table-wrap {
		overflow-x: auto;
	}

	.sort {
		display: inline-flex;
		align-items: center;
		gap: 0.2rem;
		background: none;
		border: 0;
		padding: 0;
		font: inherit;
		text-transform: inherit;
		letter-spacing: inherit;
		color: inherit;
		cursor: pointer;
	}

	.sort:hover {
		color: var(--text);
	}

	.sort :global(.flip) {
		transform: rotate(180deg);
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
		flex-wrap: wrap;
		gap: 0.3rem;
		margin-top: 0.25rem;
	}

	.num {
		text-align: right;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}

	.num .sort {
		justify-content: flex-end;
	}

	.mem-col {
		min-width: 9rem;
		width: 12rem;
	}

	.quota {
		color: var(--subtle);
	}

	.mem-val {
		display: block;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
		margin-bottom: 0.3rem;
		font-size: 0.88rem;
	}

	.spark {
		width: 8rem;
		min-width: 6rem;
	}

	.actions {
		text-align: right;
		white-space: nowrap;
	}

	.icon-link {
		display: inline-grid;
		place-items: center;
		vertical-align: middle;
		width: 2rem;
		height: 2rem;
		margin-left: 0.25rem;
		border-radius: var(--radius-sm);
		color: var(--muted);
	}

	.icon-link:hover {
		background: var(--surface-3);
		color: var(--text);
	}

	@media (max-width: 900px) {
		.hide-md {
			display: none;
		}
	}
</style>
