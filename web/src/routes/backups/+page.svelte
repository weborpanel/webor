<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import StatusPill from '#lib/components/StatusPill.svelte';
	import ScheduleField from '#lib/components/ScheduleField.svelte';
	import { request, errorMessage } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';
	import { ApprovalFlow } from '#lib/approval.svelte.ts';
	import ApprovalBanner from '#lib/components/ApprovalBanner.svelte';
	import { describeCron, isValidCron } from '#lib/schedule.ts';

	type BackupSettings = {
		enabled?: boolean;
		configured?: boolean;
		restic_installed?: boolean;
		hint?: string;
		repository?: string;
		schedule?: string;
		s3?: { endpoint?: string; region?: string; bucket?: string; prefix?: string };
		retention?: { keep_daily?: number; keep_weekly?: number };
		include_logs?: boolean;
	};

	type ManifestSite = { name: string; parent?: string; type?: string };
	type Snapshot = {
		id?: string;
		short_id?: string;
		time?: string;
		hostname?: string;
		sites?: ManifestSite[];
	};

	let settings = $state.raw<BackupSettings | null>(null);
	let snapshots = $state.raw<Snapshot[]>([]);
	let loading = $state(true);
	let loadingSnapshots = $state(false);
	let saving = $state(false);
	let running = $state(false);
	let restoring = $state(false);
	let editing = $state(false);

	let endpoint = $state('https://s3.amazonaws.com');
	let region = $state('');
	let bucket = $state('');
	let prefix = $state('');
	let schedule = $state('0 3 * * *');
	let timezone = $state('');
	let keepDaily = $state(7);
	let keepWeekly = $state(4);
	let includeLogs = $state(false);
	let accessKey = $state('');
	let secretKey = $state('');
	let resticPassword = $state('');

	let selectedSnap = $state('');
	let selectedSites = $state.raw<string[]>([]);
	let includePanel = $state(false);

	type ApprAction =
		| { kind: 'configure' }
		| { kind: 'restore'; snapshot_id: string; sites: string[]; include_panel: boolean };

	const approval = new ApprovalFlow<ApprAction>({
		operationFor: (a) => (a.kind === 'configure' ? 'BackupConfigure' : 'BackupRestore'),
		argsFor: (op, a) => {
			if (op === 'BackupConfigure' || a?.kind === 'configure') {
				return {
					endpoint: endpoint.trim(),
					region: region.trim(),
					bucket: bucket.trim(),
					prefix: prefix.trim()
				};
			}
			if (a?.kind === 'restore') {
				return {
					snapshot_id: a.snapshot_id,
					sites: a.sites,
					include_panel: a.include_panel
				};
			}
			return {};
		},
		resume: async (action, approvalId) => {
			if (action.kind === 'configure') await saveConfigure(approvalId);
			else await doRestore(approvalId, action);
		},
		onError: (m) => toasts.error(m)
	});

	let activeSites = $derived.by(() => {
		const snap = snapshots.find((s) => s.id === selectedSnap || s.short_id === selectedSnap);
		return snap?.sites ?? [];
	});

	let showForm = $derived(!settings?.configured || editing);

	function syncForm(s: BackupSettings) {
		endpoint = s.s3?.endpoint || 'https://s3.amazonaws.com';
		region = s.s3?.region || '';
		bucket = s.s3?.bucket || '';
		prefix = s.s3?.prefix || '';
		schedule = s.schedule || '0 3 * * *';
		keepDaily = s.retention?.keep_daily ?? 7;
		keepWeekly = s.retention?.keep_weekly ?? 4;
		includeLogs = !!s.include_logs;
	}

	async function loadSnapshots() {
		loadingSnapshots = true;
		try {
			const sn = await request<{ snapshots?: Snapshot[] }>('/api/backup/snapshots');
			if (sn.ok) snapshots = sn.data.snapshots ?? [];
			else snapshots = [];
		} catch {
			snapshots = [];
		} finally {
			loadingSnapshots = false;
		}
	}

	async function load() {
		void request<{ timezone?: string }>('/api/panel').then((pr) => {
			if (pr.ok && pr.data.timezone) timezone = pr.data.timezone;
		});
		loading = true;
		try {
			const r = await request<BackupSettings>('/api/backup');
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not load backup settings');
				return;
			}
			settings = r.data;
			syncForm(r.data);
			if (r.data.configured || r.data.enabled) {
				await loadSnapshots();
			} else {
				snapshots = [];
			}
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			loading = false;
		}
	}

	function startEdit() {
		if (settings) syncForm(settings);
		accessKey = '';
		secretKey = '';
		resticPassword = '';
		editing = true;
	}

	function cancelEdit() {
		if (settings) syncForm(settings);
		accessKey = '';
		secretKey = '';
		resticPassword = '';
		editing = false;
	}

	async function saveConfigure(approvalId?: string) {
		const trimmedSchedule = schedule.trim();
		if (!trimmedSchedule || !isValidCron(trimmedSchedule)) {
			toasts.error('Enter a valid 5-field cron schedule');
			return;
		}
		saving = true;
		try {
			const r = await request('/api/backup', {
				method: 'PUT',
				json: {
					endpoint: endpoint.trim(),
					region: region.trim(),
					bucket: bucket.trim(),
					prefix: prefix.trim(),
					schedule: trimmedSchedule,
					keep_daily: keepDaily,
					keep_weekly: keepWeekly,
					include_logs: includeLogs,
					access_key: accessKey,
					secret_key: secretKey,
					restic_password: resticPassword,
					approval_id: approvalId || undefined
				}
			});
			if (!r.ok) {
				if (r.status === 403 || r.data.approval_operation) {
					await approval.begin(r.data, { kind: 'configure' });
					return;
				}
				toasts.error(r.data.error || 'Configure failed');
				return;
			}
			approval.clear();
			accessKey = '';
			secretKey = '';
			resticPassword = '';
			editing = false;
			toasts.success('Backup storage configured');
			await load();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			saving = false;
		}
	}

	async function runBackup() {
		running = true;
		try {
			const r = await request<{ snapshot_id?: string }>('/api/backup/run', { method: 'POST' });
			if (!r.ok) {
				toasts.error(r.data.error || 'Backup failed');
				return;
			}
			toasts.success(r.data.snapshot_id ? `Snapshot ${r.data.snapshot_id.slice(0, 8)}…` : 'Backup complete');
			await loadSnapshots();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			running = false;
		}
	}

	function toggleSite(name: string) {
		if (selectedSites.includes(name)) {
			selectedSites = selectedSites.filter((s) => s !== name);
		} else {
			selectedSites = [...selectedSites, name];
		}
	}

	async function startRestore() {
		if (!selectedSnap) {
			toasts.error('Choose a snapshot');
			return;
		}
		if (!selectedSites.length && !includePanel) {
			toasts.error('Select at least one site or panel state');
			return;
		}
		const action: ApprAction = {
			kind: 'restore',
			snapshot_id: selectedSnap,
			sites: selectedSites,
			include_panel: includePanel
		};
		await doRestore(undefined, action);
	}

	async function doRestore(approvalId: string | undefined, action: Extract<ApprAction, { kind: 'restore' }>) {
		restoring = true;
		try {
			const r = await request('/api/backup/restore', {
				method: 'POST',
				json: {
					snapshot_id: action.snapshot_id,
					sites: action.sites,
					include_panel: action.include_panel,
					approval_id: approvalId || undefined
				}
			});
			if (!r.ok) {
				if (r.status === 403 || r.data.approval_operation) {
					await approval.begin(r.data, action);
					return;
				}
				toasts.error(r.data.error || 'Restore failed');
				return;
			}
			approval.clear();
			toasts.success('Restore complete');
			selectedSites = [];
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			restoring = false;
		}
	}

	onMount(() => {
		void load();
	});
</script>

<PageHeader title="Backups" subtitle="Full-system restic snapshots to your S3-compatible bucket. Restore picks sites." />

{#if approval.active}
	<ApprovalBanner flow={approval} />
{/if}

{#if loading && !settings}
	<Skeleton lines={6} />
{:else}
	<div class="layout" class:refreshing={loading && !!settings} aria-busy={loading}>
		{#if loading && settings}
			<p class="refresh" role="status">Refreshing…</p>
		{/if}

		<Card title="Storage" description="BYO bucket (AWS, R2, MinIO). Credentials are stored on the host, not in config.yaml.">
			{#if settings && !settings.restic_installed}
				<div class="warn" role="status">
					<p>Install restic on the host:</p>
					{#if settings.hint}
						<pre class="cmd">{settings.hint}</pre>
					{:else}
						<p>
							<code>sudo apt-get install -y restic</code>
							or
							<code>sudo dnf install -y epel-release restic</code>
						</p>
					{/if}
				</div>
			{/if}

			{#if showForm}
				<form
					class="form"
					onsubmit={(e) => {
						e.preventDefault();
						void saveConfigure();
					}}
				>
					<label class="wbr-field">
						<span>Endpoint</span>
						<input bind:value={endpoint} placeholder="https://s3.amazonaws.com" spellcheck="false" />
					</label>
					<label class="wbr-field">
						<span>Region</span>
						<input bind:value={region} placeholder="ap-south-1" spellcheck="false" />
					</label>
					<label class="wbr-field">
						<span>Bucket</span>
						<input bind:value={bucket} required spellcheck="false" />
					</label>
					<label class="wbr-field">
						<span>Prefix (optional)</span>
						<input bind:value={prefix} placeholder="prod/vps-1" spellcheck="false" />
					</label>

					<ScheduleField bind:value={schedule} timezone={timezone} />

					<div class="row">
						<label class="wbr-field">
							<span>Keep daily</span>
							<input type="number" min="1" bind:value={keepDaily} />
						</label>
						<label class="wbr-field">
							<span>Keep weekly</span>
							<input type="number" min="1" bind:value={keepWeekly} />
						</label>
					</div>
					<label class="toggle-field">
						<input type="checkbox" bind:checked={includeLogs} />
						<span>
							Include log files in backups
							<small class="hint">Site logs under each site’s logs/ plus /var/log/webor. Off by default to keep snapshots smaller.</small>
						</span>
					</label>
					<label class="wbr-field">
						<span>Access key</span>
						<input bind:value={accessKey} autocomplete="off" spellcheck="false" />
						{#if settings?.configured}
							<span class="hint">Leave blank to keep the existing key.</span>
						{/if}
					</label>
					<label class="wbr-field">
						<span>Secret key</span>
						<input type="password" bind:value={secretKey} autocomplete="new-password" />
						{#if settings?.configured}
							<span class="hint">Leave blank to keep the existing key.</span>
						{/if}
					</label>
					<label class="wbr-field">
						<span>Restic password</span>
						<input type="password" bind:value={resticPassword} autocomplete="new-password" />
						<span class="hint">
							{settings?.configured
								? 'Leave blank to keep the existing password.'
								: 'Encrypts the repository. Required on first configure.'}
						</span>
					</label>
					{#if settings?.repository}
						<p class="repo"><code>{settings.repository}</code></p>
					{/if}
					<div class="form-actions">
						<button class="wbr-btn" type="submit" disabled={saving}>
							<Icon name="check" size={15} />
							{saving ? 'Saving…' : 'Save (requires approval)'}
						</button>
						{#if settings?.configured}
							<button class="wbr-btn wbr-btn-ghost" type="button" disabled={saving} onclick={cancelEdit}>
								Cancel
							</button>
						{/if}
					</div>
				</form>
			{:else if settings}
				<div class="summary">
					<div class="summary-head">
						{#if !settings.restic_installed}
							<StatusPill tone="warn" label="restic missing" />
						{:else if settings.enabled || settings.configured}
							<StatusPill tone="ok" label="Configured" />
						{:else}
							<StatusPill tone="neutral" label="Not configured" />
						{/if}
					</div>
					<dl class="facts">
						<div>
							<dt>Bucket</dt>
							<dd>{settings.s3?.bucket || '—'}</dd>
						</div>
						<div>
							<dt>Endpoint</dt>
							<dd>{settings.s3?.endpoint || '—'}</dd>
						</div>
						{#if settings.s3?.region}
							<div>
								<dt>Region</dt>
								<dd>{settings.s3.region}</dd>
							</div>
						{/if}
						{#if settings.s3?.prefix}
							<div>
								<dt>Prefix</dt>
								<dd>{settings.s3.prefix}</dd>
							</div>
						{/if}
						<div>
							<dt>Schedule</dt>
							<dd>
								{describeCron(settings.schedule, timezone || undefined)}
								{#if settings.schedule}
									<code class="cron-raw">{settings.schedule}</code>
								{/if}
							</dd>
						</div>
						<div>
							<dt>Retention</dt>
							<dd>
								{settings.retention?.keep_daily ?? 7} daily · {settings.retention?.keep_weekly ?? 4} weekly
							</dd>
						</div>
						<div>
							<dt>Include logs</dt>
							<dd>{settings.include_logs ? 'Yes' : 'No'}</dd>
						</div>
						{#if settings.repository}
							<div class="wide">
								<dt>Repository</dt>
								<dd><code>{settings.repository}</code></dd>
							</div>
						{/if}
					</dl>
					<button class="wbr-btn wbr-btn-ghost" type="button" onclick={startEdit}>
						<Icon name="pencil" size={15} />
						Edit
					</button>
				</div>
			{/if}
		</Card>

		<div class="stack">
			<Card title="Run backup" description="Always a full snapshot of this VPS.">
				<button class="wbr-btn" type="button" disabled={running || !settings?.configured} onclick={runBackup}>
					<Icon name="upload" size={15} />
					{running ? 'Backing up…' : 'Backup now'}
				</button>
				{#if !settings?.configured}
					<p class="hint">Configure storage first.</p>
				{/if}
			</Card>

			<Card title="Restore" description="Pick a snapshot, then choose sites. Requires root approval.">
				{#if loadingSnapshots}
					<Skeleton lines={4} />
				{:else if !snapshots.length}
					<EmptyState icon="server" title="No snapshots yet" description="Run a backup after configuring S3." />
				{:else}
					<label class="wbr-field">
						<span>Snapshot</span>
						<select
							bind:value={selectedSnap}
							onchange={() => {
								selectedSites = [];
							}}
						>
							<option value="">Choose…</option>
							{#each snapshots as s (s.id || s.short_id)}
								<option value={s.id || s.short_id}>
									{(s.short_id || s.id?.slice(0, 8) || '?') +
										' — ' +
										(s.time ? new Date(s.time).toLocaleString() : '') +
										(s.hostname ? ` (${s.hostname})` : '')}
								</option>
							{/each}
						</select>
					</label>
					{#if activeSites.length}
						<div class="sites">
							{#each activeSites as site (site.name)}
								<label class="check">
									<input
										type="checkbox"
										checked={selectedSites.includes(site.name)}
										onchange={() => toggleSite(site.name)}
									/>
									<span>{site.name}{#if site.type}<small> · {site.type}</small>{/if}</span>
								</label>
							{/each}
						</div>
					{:else if selectedSnap}
						<p class="hint">Manifest sites unavailable — enter restores still work if you know names.</p>
					{/if}
					<label class="check">
						<input type="checkbox" bind:checked={includePanel} />
						<span>Include panel state (SQLite)</span>
					</label>
					<button class="wbr-btn" type="button" disabled={restoring || !selectedSnap} onclick={startRestore}>
						<Icon name="download" size={15} />
						{restoring ? 'Restoring…' : 'Restore selected'}
					</button>
				{/if}
			</Card>
		</div>
	</div>
{/if}

<style>
	.layout {
		display: grid;
		grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr);
		gap: 1rem;
		align-items: start;
		position: relative;
	}
	.layout.refreshing {
		opacity: 0.72;
		pointer-events: none;
	}
	.refresh {
		position: absolute;
		top: -0.15rem;
		right: 0;
		margin: 0;
		font-size: 0.78rem;
		font-weight: 600;
		color: var(--muted);
		z-index: 1;
	}
	.stack {
		display: grid;
		gap: 1rem;
	}
	.form {
		display: grid;
		gap: 0.75rem;
	}
	.form-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
	.row {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.75rem;
	}
	.hint {
		margin: 0.35rem 0 0;
		font-size: 0.8rem;
		color: var(--muted);
	}
	.toggle-field {
		display: flex;
		align-items: flex-start;
		gap: 0.65rem;
		font-size: 0.9rem;
		font-weight: 600;
		color: var(--text);
	}
	.toggle-field input {
		margin-top: 0.2rem;
	}
	.toggle-field .hint {
		display: block;
		font-weight: 400;
	}
	.warn {
		margin: 0 0 0.85rem;
		padding: 0.7rem 0.85rem;
		border-radius: var(--radius);
		background: var(--warn-soft, #fef3c7);
		color: var(--text);
		font-size: 0.9rem;
	}
	.warn p {
		margin: 0;
	}
	.warn .cmd {
		margin: 0.4rem 0 0;
		padding: 0.5rem 0.65rem;
		overflow: auto;
		font-size: 0.8rem;
		white-space: pre-wrap;
		background: var(--surface, #fff);
		border-radius: calc(var(--radius) - 2px);
	}
	.repo {
		margin: 0;
		font-size: 0.8rem;
		word-break: break-all;
		color: var(--muted);
	}
	.summary {
		display: grid;
		gap: 0.85rem;
	}
	.summary-head {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.facts {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.65rem 1rem;
		margin: 0;
	}
	.facts .wide {
		grid-column: 1 / -1;
	}
	.facts dt {
		margin: 0;
		font-size: 0.72rem;
		font-weight: 700;
		letter-spacing: 0.02em;
		text-transform: uppercase;
		color: var(--muted);
	}
	.facts dd {
		margin: 0.15rem 0 0;
		font-size: 0.9rem;
		word-break: break-word;
	}
	.cron-raw {
		margin-left: 0.35rem;
		font-size: 0.78rem;
		color: var(--muted);
	}
	.sites {
		display: grid;
		gap: 0.35rem;
		margin: 0.5rem 0 0.75rem;
		max-height: 12rem;
		overflow: auto;
	}
	.check {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
		font-size: 0.88rem;
	}
	.check small {
		color: var(--muted);
	}
	@media (max-width: 960px) {
		.layout {
			grid-template-columns: 1fr;
		}
	}
</style>
