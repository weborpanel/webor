<script lang="ts">
	import { onMount } from 'svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import ApiLogPanel from '#lib/components/ApiLogPanel.svelte';
	import CronSetupWizard from '#lib/components/CronSetupWizard.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';
	import { confirmDialog } from '#lib/stores/confirm.svelte.ts';

	type CronInfo = {
		name?: string;
		schedule?: string;
		exec?: string;
		dir?: string;
		unit?: string;
		timer_active?: boolean;
		last_result?: string;
		next_elapse?: string;
		failed?: boolean;
	};

	const site = getSite();

	let items = $state.raw<CronInfo[]>([]);
	let loading = $state(true);
	let loadError = $state('');
	let showAdd = $state(false);
	let selected = $state('');
	let busy = $state(false);
	let logUnit = $state('');
	let nodeChoices = $state.raw<{ id: string; label: string }[]>([]);

	let selectedItem = $derived(items.find((c) => c.name === selected) ?? null);
	let logOptions = $derived(
		selected ? [{ value: `cron:${selected}`, label: `Cron · ${selected}` }] : []
	);

	async function load() {
		loading = true;
		loadError = '';
		try {
			const r = await request<{ cron?: CronInfo[] }>(site.path('/cron'));
			if (!r.ok) {
				loadError = r.data.error || 'Could not load cron jobs';
				items = [];
				return;
			}
			items = r.data.cron ?? [];
			if (selected && !items.some((c) => c.name === selected)) {
				selected = items[0]?.name || '';
			} else if (!selected && items.length) {
				selected = items[0].name || '';
			}
			if (selected) logUnit = `cron:${selected}`;
		} catch (e) {
			loadError = errorMessage(e);
		} finally {
			loading = false;
		}
	}

	async function runNow(name: string) {
		busy = true;
		try {
			const r = await request(site.path(`/cron/${encodeURIComponent(name)}/run`), {
				method: 'POST'
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Run failed');
				return;
			}
			selected = name;
			logUnit = `cron:${name}`;
			toasts.success('Started — watch logs');
			await load();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			busy = false;
		}
	}

	async function toggleTimer(c: CronInfo, enable: boolean) {
		if (!c.name) return;
		if (enable && !c.schedule) {
			toasts.error('Set a schedule before enabling the timer');
			return;
		}
		busy = true;
		try {
			const path = enable ? 'enable' : 'disable';
			const r = await request(site.path(`/cron/${encodeURIComponent(c.name)}/${path}`), {
				method: 'POST'
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Timer update failed');
				return;
			}
			toasts.success(enable ? 'Timer enabled' : 'Timer disabled');
			await load();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			busy = false;
		}
	}

	async function remove(name: string) {
		const ok = await confirmDialog({
			title: 'Delete cron job?',
			message: `Remove ${name} and its systemd units? Files under crons/${name}/ are kept.`,
			confirmLabel: 'Delete'
		});
		if (!ok) return;
		busy = true;
		try {
			const r = await request(site.path(`/cron/${encodeURIComponent(name)}`), {
				method: 'DELETE'
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Delete failed');
				return;
			}
			toasts.success('Cron removed');
			if (selected === name) selected = '';
			await load();
			await site.load();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			busy = false;
		}
	}

	onMount(() => {
		void load();
		void request<{ versions?: { id: string; label: string }[] }>('/api/runtimes/node').then((r) => {
			if (r.ok) nodeChoices = r.data.versions ?? [];
		});
	});
</script>

{#if showAdd}
	<Card title="Add cron" description="One-shot seed/migrate or a scheduled job.">
		<CronSetupWizard
			{nodeChoices}
			ondone={async (name) => {
				showAdd = false;
				selected = name;
				await load();
				await site.load();
			}}
			oncancel={() => (showAdd = false)}
		/>
	</Card>
{:else}
	<div class="head">
		<div>
			<h2>Crons</h2>
			<p>Foreground one-shots (seed, migrate) and optional schedules. Run once and watch logs.</p>
		</div>
		<button type="button" class="wbr-btn" disabled={busy} onclick={() => (showAdd = true)}>
			<Icon name="plus" size={15} /> Add cron
		</button>
	</div>

	{#if loading}
		<Skeleton lines={4} />
	{:else if loadError}
		<p class="err">{loadError}</p>
	{:else if !items.length}
		<EmptyState
			icon="clock"
			title="No cron jobs"
			description="Add a seed script or a nightly cleanup timer."
		>
			<button type="button" class="wbr-btn" onclick={() => (showAdd = true)}>
				<Icon name="plus" size={15} /> Add cron
			</button>
		</EmptyState>
	{:else}
		<div class="layout">
			<ul class="list">
				{#each items as c (c.name)}
					<li class={['row', c.name === selected && 'on']}>
						<button type="button" class="pick" onclick={() => {
							selected = c.name || '';
							logUnit = `cron:${c.name}`;
						}}>
							<span class="name">{c.name}</span>
							<span class="meta">
								{#if c.schedule}
									{c.schedule}
									{#if c.timer_active}
										· timer on
									{:else}
										· timer off
									{/if}
								{:else}
									Run once only
								{/if}
							</span>
							{#if c.failed}<span class="badge bad">failed</span>{/if}
						</button>
					</li>
				{/each}
			</ul>

			{#if selectedItem}
				<Card title={selectedItem.name || 'Cron'} description={selectedItem.exec || ''}>
					<div class="actions">
						<button type="button" class="wbr-btn" disabled={busy} onclick={() => selectedItem?.name && void runNow(selectedItem.name)}>
							<Icon name="play" size={14} /> Run once
						</button>
						{#if selectedItem.schedule}
							{#if selectedItem.timer_active}
								<button type="button" class="wbr-btn wbr-btn-ghost" disabled={busy} onclick={() => void toggleTimer(selectedItem, false)}>
									Disable timer
								</button>
							{:else}
								<button type="button" class="wbr-btn wbr-btn-ghost" disabled={busy} onclick={() => void toggleTimer(selectedItem, true)}>
									Enable timer
								</button>
							{/if}
						{/if}
						<button type="button" class="wbr-btn wbr-btn-ghost" disabled={busy} onclick={() => selectedItem?.name && void remove(selectedItem.name)}>
							<Icon name="trash" size={14} /> Delete
						</button>
					</div>
					{#if selectedItem.next_elapse}
						<p class="hint">Next: {selectedItem.next_elapse}</p>
					{/if}
					{#if logUnit}
						<div class="logs">
							<ApiLogPanel bind:unit={logUnit} unitOptions={logOptions} compact follow={true} />
						</div>
					{/if}
				</Card>
			{/if}
		</div>
	{/if}
{/if}

<style>
	.head {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 1rem;
		margin-bottom: 1.25rem;
	}
	.head h2 {
		margin: 0 0 0.25rem;
	}
	.head p {
		margin: 0;
		opacity: 0.75;
		font-size: 0.9rem;
	}
	.err {
		color: var(--wbr-danger, #dc2626);
	}
	.layout {
		display: grid;
		gap: 1rem;
		grid-template-columns: minmax(12rem, 16rem) 1fr;
	}
	@media (max-width: 800px) {
		.layout {
			grid-template-columns: 1fr;
		}
	}
	.list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.35rem;
	}
	.pick {
		width: 100%;
		text-align: left;
		display: grid;
		gap: 0.15rem;
		padding: 0.65rem 0.75rem;
		border: 1px solid var(--wbr-border, #ccc);
		border-radius: 8px;
		background: transparent;
		cursor: pointer;
	}
	.row.on .pick {
		border-color: var(--wbr-accent, #2563eb);
	}
	.name {
		font-weight: 600;
	}
	.meta {
		font-size: 0.8rem;
		opacity: 0.7;
	}
	.badge {
		font-size: 0.75rem;
	}
	.badge.bad {
		color: var(--wbr-danger, #dc2626);
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin-bottom: 0.75rem;
	}
	.hint {
		margin: 0 0 0.75rem;
		font-size: 0.85rem;
		opacity: 0.75;
	}
	.logs {
		margin-top: 0.5rem;
	}
</style>
