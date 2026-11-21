<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import ActivityFeed from '#lib/components/ActivityFeed.svelte';
	import { api, errorMessage } from '#lib/api.ts';
	import { eventFailed, eventSite, humanizeOperation } from '#lib/activity.ts';
	import type { AuditEvent } from '#lib/types.ts';

	const PAGE = 50;

	let events = $state.raw<AuditEvent[]>([]);
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state('');
	let done = $state(false);
	let query = $state('');
	let site = $state('');
	let failedOnly = $state(false);
	let detailed = $state(false);

	let sites = $derived([...new Set(events.map(eventSite).filter(Boolean))].sort());

	let filtered = $derived.by(() => {
		const q = query.trim().toLowerCase();
		return events.filter((e) => {
			if (failedOnly && !eventFailed(e)) return false;
			if (site && eventSite(e) !== site) return false;
			if (!q) return true;
			return (
				e.operation.toLowerCase().includes(q) ||
				humanizeOperation(e.operation).toLowerCase().includes(q) ||
				e.actor.toLowerCase().includes(q) ||
				JSON.stringify(e.args ?? {}).toLowerCase().includes(q)
			);
		});
	});

	let groups = $derived.by(() => {
		const out: { label: string; events: AuditEvent[] }[] = [];
		const today = new Date().toDateString();
		const yesterday = new Date(Date.now() - 86_400_000).toDateString();
		for (const e of filtered) {
			const d = new Date(e.ts);
			const ds = d.toDateString();
			const label =
				ds === today
					? 'Today'
					: ds === yesterday
						? 'Yesterday'
						: d.toLocaleDateString(undefined, { weekday: 'long', month: 'short', day: 'numeric', year: 'numeric' });
			const last = out[out.length - 1];
			if (last?.label === label) last.events.push(e);
			else out.push({ label, events: [e] });
		}
		return out;
	});

	let filtering = $derived(!!query.trim() || !!site || failedOnly);

	async function load(more = false) {
		if (more) loadingMore = true;
		else loading = true;
		try {
			const before = more && events.length ? `&before=${events[events.length - 1].id}` : '';
			const data = await api<{ events?: AuditEvent[] }>(`/api/audit?limit=${PAGE}${before}`);
			const batch = data.events ?? [];
			events = more ? [...events, ...batch] : batch;
			done = batch.length < PAGE;
			error = '';
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
			loadingMore = false;
		}
	}

	function clearFilters() {
		query = '';
		site = '';
		failedOnly = false;
	}

	onMount(() => {
		void load();
	});
</script>

<svelte:head>
	<title>Activity · Webor</title>
</svelte:head>

<PageHeader title="Activity" subtitle="Audit log of privileged operations performed by the panel and helper">
	{#snippet actions()}
		<button type="button" class="wbr-btn wbr-btn-ghost" disabled={loading} onclick={() => load()}>
			<Icon name="refresh" size={15} /> Refresh
		</button>
	{/snippet}
</PageHeader>

<div class="filters">
	<label class="search">
		<Icon name="search" size={15} />
		<span class="visually-hidden">Search activity</span>
		<input bind:value={query} placeholder="Search operations, sites, actors…" spellcheck="false" />
	</label>
	<label class="wbr-field">
		<span class="visually-hidden">Site</span>
		<select bind:value={site}>
			<option value="">All sites</option>
			{#each sites as s (s)}
				<option value={s}>{s}</option>
			{/each}
		</select>
	</label>
	<label class="toggle"><input type="checkbox" bind:checked={failedOnly} /> Failures only</label>
	<label class="toggle"><input type="checkbox" bind:checked={detailed} /> Show details</label>
</div>

{#if error}<p class="wbr-error" role="alert">{error}</p>{/if}

{#if loading && events.length === 0}
	<Card><Skeleton lines={8} /></Card>
{:else if events.length === 0}
	<Card>
		<EmptyState icon="activity" title="No activity yet" description="Privileged operations such as creating sites, databases and certificates show up here." />
	</Card>
{:else if filtered.length === 0}
	<Card>
		<EmptyState icon="search" title="No matching events" description="Try a different search or clear the filters." compact>
			<button type="button" class="wbr-btn wbr-btn-ghost" onclick={clearFilters}>Clear filters</button>
		</EmptyState>
	</Card>
{:else}
	<div class="groups">
		{#each groups as g (g.label)}
			<section aria-label={g.label}>
				<h2>{g.label}</h2>
				<Card>
					<ActivityFeed events={g.events} {detailed} />
				</Card>
			</section>
		{/each}
	</div>
{/if}

{#if events.length > 0}
	<div class="more">
		{#if done}
			<span class="wbr-muted">
				{filtering ? `${filtered.length} of ${events.length} loaded events` : `All ${events.length} events loaded`}
			</span>
		{:else}
			<button type="button" class="wbr-btn wbr-btn-ghost" disabled={loadingMore} onclick={() => load(true)}>
				{loadingMore ? 'Loading…' : 'Load older events'}
			</button>
		{/if}
	</div>
{/if}

<style>
	.filters {
		display: flex;
		flex-wrap: wrap;
		gap: 0.6rem;
		align-items: center;
		margin-bottom: 1.1rem;
	}

	.search {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		flex: 1;
		min-width: 14rem;
		max-width: 26rem;
		padding: 0 0.7rem;
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
		border: 0;
		outline: 0;
		background: transparent;
		color: var(--text);
		font: inherit;
		font-size: 0.9rem;
		padding: 0.5rem 0;
	}

	.toggle {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.86rem;
		font-weight: 600;
		color: var(--muted);
	}

	.groups {
		display: grid;
		gap: 1.1rem;
	}

	h2 {
		margin: 0 0 0.45rem;
		font-size: 0.78rem;
		font-weight: 800;
		color: var(--subtle);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.more {
		display: flex;
		justify-content: center;
		padding: 1.25rem 0 0.5rem;
		font-size: 0.86rem;
	}
</style>
