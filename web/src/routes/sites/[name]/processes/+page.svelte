<script lang="ts">
	import { onMount } from 'svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import ApiLogPanel from '#lib/components/ApiLogPanel.svelte';
	import ProcessSetupWizard from '#lib/components/ProcessSetupWizard.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';
	import { confirmDialog } from '#lib/stores/confirm.svelte.ts';

	type ProcessInfo = {
		name?: string;
		exec?: string;
		dir?: string;
		unit?: string;
		active?: boolean;
		failed?: boolean;
	};

	const site = getSite();

	let items = $state.raw<ProcessInfo[]>([]);
	let loading = $state(true);
	let loadError = $state('');
	let showAdd = $state(false);
	let selected = $state('');
	let busy = $state(false);
	let logUnit = $state('');

	let selectedItem = $derived(items.find((p) => p.name === selected) ?? null);
	let logOptions = $derived(
		selected ? [{ value: `job:${selected}`, label: `Process · ${selected}` }] : []
	);

	async function load() {
		loading = true;
		loadError = '';
		try {
			const r = await request<{ processes?: ProcessInfo[] }>(site.path('/processes'));
			if (!r.ok) {
				loadError = r.data.error || 'Could not load processes';
				items = [];
				return;
			}
			items = r.data.processes ?? [];
			if (selected && !items.some((p) => p.name === selected)) {
				selected = items[0]?.name || '';
			} else if (!selected && items.length) {
				selected = items[0].name || '';
			}
			if (selected) logUnit = `job:${selected}`;
		} catch (e) {
			loadError = errorMessage(e);
		} finally {
			loading = false;
		}
	}

	async function ctrl(name: string, action: 'start' | 'stop' | 'restart') {
		busy = true;
		try {
			const r = await request(site.path(`/processes/${encodeURIComponent(name)}/${action}`), {
				method: 'POST'
			});
			if (!r.ok) {
				toasts.error(r.data.error || `${action} failed`);
				return;
			}
			selected = name;
			logUnit = `job:${name}`;
			toasts.success(action === 'stop' ? 'Stopped' : action === 'start' ? 'Started' : 'Restarted');
			await load();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			busy = false;
		}
	}

	async function remove(name: string) {
		const ok = await confirmDialog({
			title: 'Delete process?',
			message: `Stop and remove ${name}. Files under processes/${name}/ are kept.`,
			confirmLabel: 'Delete'
		});
		if (!ok) return;
		busy = true;
		try {
			const r = await request(site.path(`/processes/${encodeURIComponent(name)}`), {
				method: 'DELETE'
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Delete failed');
				return;
			}
			toasts.success('Process removed');
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
	});
</script>

{#if showAdd}
	<Card title="Add process" description="Long-running background worker or queue.">
		<ProcessSetupWizard
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
			<h2>Processes</h2>
			<p>Background services (queues, workers). Start stays up until you Stop.</p>
		</div>
		<button type="button" class="wbr-btn" disabled={busy} onclick={() => (showAdd = true)}>
			<Icon name="plus" size={15} /> Add process
		</button>
	</div>

	{#if loading}
		<Skeleton lines={4} />
	{:else if loadError}
		<p class="err">{loadError}</p>
	{:else if !items.length}
		<EmptyState
			icon="activity"
			title="No processes"
			description="Add a queue worker or other long-running job."
		>
			<button type="button" class="wbr-btn" onclick={() => (showAdd = true)}>
				<Icon name="plus" size={15} /> Add process
			</button>
		</EmptyState>
	{:else}
		<div class="layout">
			<ul class="list">
				{#each items as p (p.name)}
					<li class={['row', p.name === selected && 'on']}>
						<button
							type="button"
							class="pick"
							onclick={() => {
								selected = p.name || '';
								logUnit = `job:${p.name}`;
							}}
						>
							<span class="name">{p.name}</span>
							<span class="meta">
								{#if p.active}
									running
								{:else if p.failed}
									failed
								{:else}
									stopped
								{/if}
							</span>
						</button>
					</li>
				{/each}
			</ul>

			{#if selectedItem}
				<Card title={selectedItem.name || 'Process'} description={selectedItem.exec || ''}>
					<div class="actions">
						{#if selectedItem.active}
							<button type="button" class="wbr-btn" disabled={busy} onclick={() => selectedItem?.name && void ctrl(selectedItem.name, 'restart')}>
								<Icon name="refresh" size={14} /> Restart
							</button>
							<button type="button" class="wbr-btn wbr-btn-ghost" disabled={busy} onclick={() => selectedItem?.name && void ctrl(selectedItem.name, 'stop')}>
								<Icon name="pause" size={14} /> Stop
							</button>
						{:else}
							<button type="button" class="wbr-btn" disabled={busy} onclick={() => selectedItem?.name && void ctrl(selectedItem.name, 'start')}>
								<Icon name="play" size={14} /> Start
							</button>
						{/if}
						<button type="button" class="wbr-btn wbr-btn-ghost" disabled={busy} onclick={() => selectedItem?.name && void remove(selectedItem.name)}>
							<Icon name="trash" size={14} /> Delete
						</button>
					</div>
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
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin-bottom: 0.75rem;
	}
	.logs {
		margin-top: 0.5rem;
	}
</style>
