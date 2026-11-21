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

	type DeleteAction = { kind: 'delete'; name: string };
	type DeletePreview = ApiBase & { removed_children?: string[]; drop_databases?: { id: string; engine: string }[] };

	let summary = $derived(overview.sites.find((s) => s.name.toLowerCase() === site.name.toLowerCase()));
	let deleting = $state(false);

	const flow = new ApprovalFlow<DeleteAction>({
		operationFor: () => 'DeleteSiteUser',
		argsFor: (_op, a) => ({ domain: a?.name ?? site.name, delete_data: true }),
		resume: (a, approvalId) => runDelete(a, approvalId),
		onError: (m) => toasts.error(m)
	});

	const configSnippet = $derived(
		[
			`sites:`,
			`  - name: ${site.name}`,
			`    limits:`,
			`      memory_max: ${summary?.memory_max || '256M'}`,
			`      disk_quota: ${summary?.disk_quota || '2G'}`
		].join('\n')
	);

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
	});
</script>

{#if flow.active}
	<ApprovalBanner {flow} detail={`delete ${site.name} and its data`} />
{/if}

<div class="grid">
	<Card title="Resource limits" description="Enforced by systemd for the site's units">
		<div class="limits">
			<div class="limit">
				<span class="label"><Icon name="memory" size={15} /> Memory</span>
				{#if summary?.memory_max}
					<strong>{summary.memory_max}</strong>
					{#if summary.mem_limit_bytes}
						<UsageBar used={summary.mem_bytes ?? 0} total={summary.mem_limit_bytes} format={formatBytes} compact />
						<small>{formatBytes(summary.mem_bytes ?? 0)} in use</small>
					{/if}
				{:else}
					<strong class="none">No limit</strong>
				{/if}
			</div>
			<div class="limit">
				<span class="label"><Icon name="disk" size={15} /> Disk quota</span>
				{#if summary?.disk_quota}
					<strong>{summary.disk_quota}</strong>
					{#if summary.disk_quota_bytes}
						<UsageBar used={summary.disk_bytes ?? 0} total={summary.disk_quota_bytes} format={formatBytes} compact />
						<small>{formatBytes(summary.disk_bytes ?? 0)} used</small>
					{/if}
				{:else}
					<strong class="none">No quota</strong>
				{/if}
			</div>
		</div>
		{#snippet footer()}
			<details class="howto">
				<summary>How to change limits</summary>
				<p class="wbr-muted">
					Edit <code>/etc/webor/config.yaml</code> on the server, then re-apply the config. The memory limit also caps
					builds.
				</p>
				<pre>{configSnippet}</pre>
			</details>
		{/snippet}
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
				<span>{site.cron.length ? site.cron.join(', ') : '—'}</span>
			</li>
			<li>
				<span>Jobs</span>
				<span>{site.jobs.length ? site.jobs.join(', ') : '—'}</span>
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

	.limits {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1.25rem;
	}

	.limit {
		display: grid;
		gap: 0.35rem;
		align-content: start;
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

	.limit strong {
		font-size: 1.25rem;
	}

	.limit .none {
		color: var(--muted);
		font-size: 1rem;
	}

	.limit small {
		font-size: 0.78rem;
		color: var(--muted);
	}

	.howto summary {
		cursor: pointer;
		font-size: 0.85rem;
		font-weight: 700;
		color: var(--accent);
	}

	.howto p {
		font-size: 0.84rem;
	}

	.howto pre {
		margin: 0;
		padding: 0.7rem 0.85rem;
		border-radius: var(--radius-sm);
		background: var(--surface-3);
		font-family: var(--mono);
		font-size: 0.78rem;
		overflow-x: auto;
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

	@media (max-width: 520px) {
		.limits {
			grid-template-columns: 1fr;
		}
	}
</style>
