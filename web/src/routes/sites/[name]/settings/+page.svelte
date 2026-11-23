<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import CopyField from '#lib/components/CopyField.svelte';
	import ApprovalBanner from '#lib/components/ApprovalBanner.svelte';
	import UsageBar from '#lib/charts/UsageBar.svelte';
	import { ApprovalFlow } from '#lib/approval.svelte.ts';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage, type ApiBase } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';
	import { confirmDialog } from '#lib/stores/confirm.svelte.ts';
	import { overview, recentSites } from '#lib/stores/overview.svelte.ts';
	import { formatBytes } from '#lib/format.ts';

	const site = getSite();

	type LimitOption = { value: string; label: string };
	type LimitsPayload = ApiBase & {
		limits?: { memory_max?: string; cpu_quota?: string; disk_quota?: string; log_max?: string };
		options?: { memory?: LimitOption[]; cpu?: LimitOption[]; disk?: LimitOption[] };
		note?: string;
	};
	type DeleteAction = { kind: 'delete'; name: string };
	type DeletePreview = ApiBase & { removed_children?: string[]; drop_databases?: { id: string; engine: string }[] };

	let summary = $derived(overview.sites.find((s) => s.name.toLowerCase() === site.name.toLowerCase()));
	let deleting = $state(false);
	let loadingLimits = $state(true);
	let saving = $state(false);
	let memoryMax = $state('');
	let cpuQuota = $state('');
	let diskQuota = $state('');
	let memoryOpts = $state<LimitOption[]>([{ value: '', label: 'No limit' }]);
	let cpuOpts = $state<LimitOption[]>([{ value: '', label: 'No limit' }]);
	let diskOpts = $state<LimitOption[]>([{ value: '', label: 'No quota' }]);
	let limitsNote = $state('');
	let customMemory = $state('');
	let customCPU = $state('');
	let customDisk = $state('');
	let blockedCustom = $derived(
		(!!customMemory && memoryMax === customMemory) ||
			(!!customCPU && cpuQuota === customCPU) ||
			(!!customDisk && diskQuota === customDisk)
	);

	const flow = new ApprovalFlow<DeleteAction>({
		operationFor: () => 'DeleteSiteUser',
		argsFor: (_op, a) => ({ domain: a?.name ?? site.name, delete_data: true }),
		resume: (a, approvalId) => runDelete(a, approvalId),
		onError: (m) => toasts.error(m)
	});

	function ensureOption(opts: LimitOption[], value: string, labelPrefix: string): LimitOption[] {
		const v = value.trim();
		if (!v) return opts;
		if (opts.some((o) => o.value.toLowerCase() === v.toLowerCase())) return opts;
		return [...opts, { value: v, label: `${labelPrefix} ${v} (config.yaml)` }];
	}

	async function loadLimits() {
		loadingLimits = true;
		try {
			const r = await request<LimitsPayload>(`/api/sites/${encodeURIComponent(site.name)}/limits`);
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not load limits');
				return;
			}
			memoryOpts = r.data.options?.memory ?? memoryOpts;
			cpuOpts = r.data.options?.cpu ?? cpuOpts;
			diskOpts = r.data.options?.disk ?? diskOpts;
			limitsNote = r.data.note ?? '';
			memoryMax = r.data.limits?.memory_max ?? '';
			cpuQuota = r.data.limits?.cpu_quota ?? '';
			diskQuota = r.data.limits?.disk_quota || '10M';
			customMemory = memoryMax && !memoryOpts.some((o) => o.value.toLowerCase() === memoryMax.toLowerCase()) ? memoryMax : '';
			customCPU = cpuQuota && !cpuOpts.some((o) => o.value === cpuQuota) ? cpuQuota : '';
			customDisk =
				diskQuota &&
				diskQuota !== '10M' &&
				!diskOpts.some((o) => o.value.toLowerCase() === diskQuota.toLowerCase())
					? diskQuota
					: '';
			memoryOpts = ensureOption(memoryOpts, customMemory, 'Custom');
			cpuOpts = ensureOption(cpuOpts, customCPU, 'Custom');
			diskOpts = ensureOption(diskOpts, customDisk, 'Custom');
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			loadingLimits = false;
		}
	}

	async function saveLimits() {
		if (blockedCustom) {
			toasts.error('Pick a panel preset, or edit config.yaml for custom / over-10G values.');
			return;
		}
		saving = true;
		try {
			const r = await request<LimitsPayload>(`/api/sites/${encodeURIComponent(site.name)}/limits`, {
				method: 'PUT',
				body: JSON.stringify({
					memory_max: memoryMax,
					cpu_quota: cpuQuota,
					disk_quota: diskQuota
				})
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not save limits');
				return;
			}
			toasts.success('Resource limits saved');
			await overview.refresh();
			await loadLimits();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			saving = false;
		}
	}

	async function runDelete(action: DeleteAction, approvalId: string = flow.id.trim()): Promise<void> {
		deleting = true;
		try {
			const q = new URLSearchParams({ delete_data: '1' });
			if (approvalId) q.set('approval_id', approvalId);
			const r = await request<DeletePreview>(`/api/sites/${encodeURIComponent(action.name)}?${q}`, {
				method: 'DELETE'
			});
			if (r.data.approval_required) {
				deleting = false;
				await flow.begin(r.data, action);
				return;
			}
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not delete site');
				return;
			}
			flow.clear();
			recentSites.forget(action.name);
			toasts.success(`${action.name} deleted`);
			await overview.refresh();
			await goto('/sites');
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			deleting = false;
		}
	}

	async function startDelete() {
		const children = site.isChild ? [] : overview.sites.filter((s) => s.parent === site.name).map((s) => s.name);
		const dbs = summary?.database_count ?? 0;
		const details = [
			`All files under /srv/sites/${site.name}/ and the site user`,
			!site.isChild && dbs ? `${dbs} database${dbs === 1 ? '' : 's'} (dropped)` : '',
			!site.isChild && summary?.cache ? 'The Valkey cache user and its keys' : '',
			children.length ? `Subdomains: ${children.join(', ')}` : ''
		].filter(Boolean);
		const ok = await confirmDialog({
			title: `Delete ${site.name}?`,
			message: 'This permanently removes:',
			details,
			confirmLabel: 'Delete site',
			danger: true,
			typeToConfirm: site.name
		});
		if (!ok) return;
		const action: DeleteAction = { kind: 'delete', name: site.name };
		flow.resetIfDifferent(action);
		await runDelete(action);
	}

	onMount(() => {
		void overview.ensure(30_000);
		void loadLimits();
	});
</script>

{#if flow.active}
	<ApprovalBanner {flow} detail={`delete ${site.name} and its data`} />
{/if}

<div class="grid">
	<Card title="Resource limits" description="Presets capped by this host. Memory and CPU apply to app units and builds.">
		{#if loadingLimits}
			<p class="wbr-muted small">Loading presets…</p>
		{:else}
			<form
				class="limits-form"
				onsubmit={(e) => {
					e.preventDefault();
					void saveLimits();
				}}
			>
				<label class="wbr-field">
					<span class="label"><Icon name="memory" size={15} /> Memory</span>
					<select class="wbr-input" bind:value={memoryMax}>
						{#each memoryOpts as opt (opt.value || 'none')}
							<option value={opt.value}>{opt.label}</option>
						{/each}
					</select>
					{#if summary?.mem_limit_bytes}
						<UsageBar used={summary.mem_bytes ?? 0} total={summary.mem_limit_bytes} format={formatBytes} compact />
						<small>{formatBytes(summary.mem_bytes ?? 0)} in use</small>
					{/if}
					{#if customMemory && memoryMax === customMemory}
						<small class="warn">Custom {customMemory} — pick a preset to change, or edit config.yaml.</small>
					{/if}
				</label>

				<label class="wbr-field">
					<span class="label"><Icon name="cpu" size={15} /> CPU</span>
					<select class="wbr-input" bind:value={cpuQuota}>
						{#each cpuOpts as opt (opt.value || 'none')}
							<option value={opt.value}>{opt.label}</option>
						{/each}
					</select>
					<small class="hint">1c = one full core (systemd CPUQuota 100%).</small>
					{#if customCPU && cpuQuota === customCPU}
						<small class="warn">Custom {customCPU} — pick a preset to change, or edit config.yaml.</small>
					{/if}
				</label>

				<label class="wbr-field">
					<span class="label"><Icon name="disk" size={15} /> Disk quota</span>
					<select class="wbr-input" bind:value={diskQuota}>
						{#each diskOpts as opt (opt.value || 'none')}
							<option value={opt.value}>{opt.label}</option>
						{/each}
					</select>
					{#if summary?.disk_quota_bytes}
						<UsageBar used={summary.disk_bytes ?? 0} total={summary.disk_quota_bytes} format={formatBytes} compact />
						<small>{formatBytes(summary.disk_bytes ?? 0)} used</small>
					{/if}
					{#if customDisk && diskQuota === customDisk}
						<small class="warn">Custom {customDisk} — above 10G stays in config.yaml; pick a preset ≤ 10G here.</small>
					{:else if limitsNote}
						<small class="hint">{limitsNote}</small>
					{/if}
				</label>

				<div class="actions">
					<button type="submit" class="wbr-btn" disabled={saving || blockedCustom}>
						{saving ? 'Saving…' : 'Save limits'}
					</button>
				</div>
			</form>
		{/if}
	</Card>

	<Card title="Runtime">
		<ul class="wbr-facts">
			<li><span>Type</span> <strong>{site.type}</strong></li>
			<li><span>Home</span> <code>/srv/sites/{site.name}/</code></li>
			{#if !site.isApp}<li><span>Web root</span> <code>{site.root}/</code></li>{/if}
			{#if site.isApp}
				<li><span>Activation</span> <strong>{site.socketProxy ? 'Socket (idle-stop)' : 'Always on'}</strong></li>
			{/if}
			<li><span>HTTPS</span> <strong>{site.https ? 'On' : 'Off'}</strong></li>
			{#if site.isChild}
				<li><span>Parent</span> <a href={`/sites/${encodeURIComponent(site.parent)}`}>{site.parent}</a></li>
			{/if}
			<li>
				<span>Cron</span>
				<span
					>{#if site.cron.length}<a href={site.href('/cron')}>{site.cron.join(', ')}</a>{:else}<a
							href={site.href('/cron')}>Manage</a
						>{/if}</span
				>
			</li>
			<li>
				<span>Processes</span>
				<span
					>{#if site.jobs.length}<a href={site.href('/processes')}>{site.jobs.join(', ')}</a>{:else}<a
							href={site.href('/processes')}>Manage</a
						>{/if}</span
				>
			</li>
		</ul>
	</Card>

	<Card title="API">
		<p class="wbr-muted small">Base path for automating this site with the panel API.</p>
		<CopyField value={site.path()} />
	</Card>

	<section class="danger" aria-labelledby="danger-title">
		<div>
			<h2 id="danger-title"><Icon name="alert" size={16} /> Delete this site</h2>
			<p>
				Removes files, the site user, {site.isChild ? 'and routes' : 'databases, cache and subdomains'}. Needs root
				approval on production servers. This cannot be undone.
			</p>
		</div>
		<button type="button" class="wbr-btn-danger solid" disabled={deleting || flow.autoBusy} onclick={startDelete}>
			<Icon name="trash" size={15} />
			{deleting || flow.autoBusy ? 'Deleting…' : 'Delete site'}
		</button>
	</section>
</div>

<style>
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(min(22rem, 100%), 1fr));
		gap: 1rem;
		align-items: start;
	}

	.limits-form {
		display: grid;
		gap: 1.1rem;
	}

	.label {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		font-size: 0.78rem;
		font-weight: 700;
		color: var(--subtle);
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.limits-form :global(.wbr-field) {
		display: grid;
		gap: 0.35rem;
	}

	.limits-form small,
	.hint {
		font-size: 0.78rem;
		color: var(--muted);
	}

	.warn {
		color: var(--danger);
	}

	.actions {
		display: flex;
		justify-content: flex-start;
	}

	.small {
		font-size: 0.84rem;
		margin: 0 0 0.6rem;
	}

	.danger {
		grid-column: 1 / -1;
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 1.1rem 1.25rem;
		border: 1px solid color-mix(in srgb, var(--danger) 40%, var(--border));
		border-radius: var(--radius-lg);
		background: var(--danger-soft);
	}

	.danger h2 {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		margin: 0 0 0.25rem;
		font-size: 1rem;
		color: var(--danger);
	}

	.danger p {
		margin: 0;
		font-size: 0.88rem;
		max-width: 42rem;
	}
</style>
