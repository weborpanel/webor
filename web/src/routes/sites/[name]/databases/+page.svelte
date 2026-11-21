<script lang="ts">
	import { onMount } from 'svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import CopyField from '#lib/components/CopyField.svelte';
	import ApprovalBanner from '#lib/components/ApprovalBanner.svelte';
	import SecretOnce from '#lib/components/SecretOnce.svelte';
	import { ApprovalFlow } from '#lib/approval.svelte.ts';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';
	import { confirmDialog } from '#lib/stores/confirm.svelte.ts';
	import { overview } from '#lib/stores/overview.svelte.ts';

	const site = getSite();

	type DbEngine = 'sqlite' | 'postgres' | 'mariadb';
	type EngineStatus = { installed?: boolean; active?: boolean } | null;
	type DbEntry = {
		id?: string;
		engine?: string;
		database?: string;
		username?: string;
		path?: string;
		peer_url?: string;
		peer_dsn?: string;
	};
	type DbAction = { kind: 'add' | 'remove'; id: string; engine: string };

	const ENGINES: { value: DbEngine; label: string; hint: string }[] = [
		{ value: 'sqlite', label: 'SQLite', hint: 'A file in the site home. No server needed.' },
		{ value: 'postgres', label: 'PostgreSQL', hint: 'Unix-socket peer auth as the site user.' },
		{ value: 'mariadb', label: 'MariaDB', hint: 'Unix-socket auth as the site user.' }
	];

	const PG_INSTALL =
		'sudo apt-get update && sudo apt-get install -y postgresql postgresql-client\nsudo systemctl enable --now postgresql';
	const PG_START = 'sudo systemctl enable --now postgresql';
	const MARIA_INSTALL =
		'sudo apt-get update && sudo apt-get install -y mariadb-server\nsudo systemctl enable --now mariadb';
	const MARIA_START = 'sudo systemctl enable --now mariadb';

	let databases = $state.raw<DbEntry[]>([]);
	let defaultId = $state('');
	let browsePath = $state('/db/');
	let postgres = $state.raw<EngineStatus>(null);
	let mariadb = $state.raw<EngineStatus>(null);
	let loading = $state(true);
	let loadError = $state('');
	let busy = $state(false);
	let rotatingId = $state('');
	let newDbId = $state('');
	let engine = $state<DbEngine>('sqlite');
	let revealed = $state<{ id: string; password: string; rotated: boolean } | null>(null);
	let installHint = $state('');

	const flow = new ApprovalFlow<DbAction>({
		operationFor: (a) => (a.kind === 'add' ? 'EnsureSiteDatabase' : 'DropSiteDatabase'),
		argsFor: (op, a) =>
			op === 'EnsureSiteDatabase' || op === 'DropSiteDatabase'
				? { domain: site.name, engine: a?.engine || engine, id: a ? a.id : normalizeId(newDbId) }
				: {},
		resume: (a, approvalId) => (a.kind === 'add' ? add(a, approvalId) : remove(a, approvalId)),
		onError: (m) => toasts.error(m)
	});

	let selectedStatus = $derived(
		engine === 'postgres' ? postgres : engine === 'mariadb' ? mariadb : null
	);
	let needsInstall = $derived(
		(engine === 'postgres' || engine === 'mariadb') && selectedStatus !== null && !selectedStatus.installed
	);
	let needsStart = $derived(
		(engine === 'postgres' || engine === 'mariadb') &&
			!!selectedStatus?.installed &&
			!selectedStatus.active
	);
	let blocked = $derived(needsInstall || needsStart);
	let engineLabel = $derived(
		engine === 'postgres' ? 'PostgreSQL' : engine === 'mariadb' ? 'MariaDB' : ''
	);
	let defaultCmdHint = $derived(
		needsInstall
			? engine === 'postgres'
				? PG_INSTALL
				: MARIA_INSTALL
			: engine === 'postgres'
				? PG_START
				: MARIA_START
	);

	function normalizeId(raw: string) {
		return raw.trim().toLowerCase() || defaultId;
	}

	function dbKey(db: DbEntry) {
		return db.id || defaultId || 'default';
	}

	function engineState(e: string): { tone: 'ok' | 'warn' | 'neutral'; text: string } | null {
		const s = e === 'postgres' ? postgres : e === 'mariadb' ? mariadb : null;
		if (e === 'sqlite') return null;
		if (!s?.installed) return { tone: 'warn', text: 'not installed' };
		return s.active ? { tone: 'ok', text: 'running' } : { tone: 'warn', text: 'installed, inactive' };
	}

	async function load() {
		loading = true;
		try {
			const r = await request<{
				databases?: DbEntry[];
				default_id?: string;
				browse_path?: string;
				postgres?: EngineStatus;
				mariadb?: EngineStatus;
			}>(site.path('/databases'));
			if (!r.ok) {
				loadError = r.data.error || 'Could not load databases';
				return;
			}
			loadError = '';
			installHint = '';
			databases = r.data.databases || [];
			defaultId = r.data.default_id || '';
			browsePath = r.data.browse_path || '/db/';
			postgres = r.data.postgres ?? null;
			mariadb = r.data.mariadb ?? null;
		} catch (e) {
			loadError = errorMessage(e);
		} finally {
			loading = false;
		}
	}

	async function add(resume?: DbAction, approvalId: string = flow.id.trim()): Promise<void> {
		if (site.isChild || (!resume && blocked)) return;
		const action: DbAction = resume ?? { kind: 'add', id: normalizeId(newDbId), engine };
		if (!resume) flow.resetIfDifferent(action);
		busy = true;
		installHint = '';
		try {
			const payload: Record<string, string> = { engine: action.engine };
			if (action.id) payload.id = action.id;
			if (approvalId) payload.approval_id = approvalId;
			const r = await request<{
				password?: string;
				install_required?: boolean;
				approval_required?: boolean;
				hint?: string;
				error?: string;
			}>(site.path('/databases'), { method: 'POST', json: payload });
			if (r.data.install_required) {
				installHint =
					r.data.hint ||
					(action.engine === 'mariadb'
						? needsStart
							? MARIA_START
							: MARIA_INSTALL
						: needsStart
							? PG_START
							: PG_INSTALL);
				toasts.error(r.data.error || 'Database engine is not ready');
				await load();
				return;
			}
			if (r.data.approval_required) {
				busy = false;
				await flow.begin(r.data, action);
				return;
			}
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not add database');
				return;
			}
			const id = action.id || defaultId || 'default';
			if (r.data.password) revealed = { id, password: r.data.password, rotated: false };
			toasts.success(`Database ${id} (${action.engine}) is ready`);
			flow.clear();
			newDbId = '';
			await load();
			void overview.refresh();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			busy = false;
		}
	}

	async function remove(action: DbAction, approvalId: string = flow.id.trim()): Promise<void> {
		if (site.isChild) return;
		busy = true;
		try {
			const r = await request(site.path(`/databases/${encodeURIComponent(action.id || defaultId)}`), {
				method: 'DELETE',
				json: approvalId ? { approval_id: approvalId } : {}
			});
			if (r.data.approval_required) {
				busy = false;
				await flow.begin(r.data, action);
				return;
			}
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not remove database');
				return;
			}
			toasts.success(`Database ${action.id || 'default'} removed`);
			flow.clear();
			await load();
			void overview.refresh();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			busy = false;
		}
	}

	async function confirmRemove(db: DbEntry) {
		const id = dbKey(db);
		const ok = await confirmDialog({
			title: `Drop database ${id}?`,
			message: `The ${db.engine || 'sqlite'} database "${id}" will be dropped from ${site.name}. Its data cannot be recovered.`,
			confirmLabel: 'Drop database',
			danger: true,
			typeToConfirm: id
		});
		if (!ok) return;
		const action: DbAction = { kind: 'remove', id, engine: db.engine || 'sqlite' };
		flow.resetIfDifferent(action);
		await remove(action);
	}

	async function rotate(db: DbEntry) {
		const id = dbKey(db);
		const ok = await confirmDialog({
			title: `Rotate password for ${id}?`,
			message: 'Apps using the old password stop working until you update their configuration.',
			confirmLabel: 'Rotate password',
			danger: true
		});
		if (!ok) return;
		rotatingId = id;
		try {
			const r = await request<{ password?: string }>(
				site.path(`/databases/${encodeURIComponent(id)}/rotate-password`),
				{ method: 'POST' }
			);
			if (r.data.approval_required) {
				toasts.error(r.data.hint || r.data.message || 'Root approval required to rotate this password');
				return;
			}
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not rotate password');
				return;
			}
			if (r.data.password) revealed = { id, password: r.data.password, rotated: true };
			toasts.success(`Password for ${id} rotated`);
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			rotatingId = '';
		}
	}

	onMount(() => {
		void load();
	});
</script>

{#if site.isChild}
	<div class="inherit" role="note">
		<Icon name="link" size={16} />
		<p>
			Databases are inherited from
			<a href={`/sites/${encodeURIComponent(site.parent)}/databases`}>{site.parent}</a>. Add, remove and
			rotate passwords on the parent site.
		</p>
	</div>
{/if}

{#if flow.active && !site.isChild}
	<ApprovalBanner
		{flow}
		detail={flow.pending ? `${flow.pending.engine} · ${flow.pending.id || 'default'}` : undefined}
	/>
{/if}

{#if revealed}
	<SecretOnce
		subject={revealed.id}
		secret={revealed.password}
		rotated={revealed.rotated}
		ondone={() => (revealed = null)}
	/>
{/if}

<div class="layout">
	<Card title="Databases" description={site.isChild ? `Shared from ${site.parent}` : 'Credentials are scoped to this site user'} flush>
		{#snippet actions()}
			<a class="wbr-btn wbr-btn-ghost sm" href={browsePath || '/db/'} target="_blank" rel="noopener noreferrer">
				<Icon name="external" size={14} /> Adminer
			</a>
			<button type="button" class="wbr-btn wbr-btn-ghost sm" disabled={loading || busy} onclick={load} aria-label="Refresh databases">
				<Icon name="refresh" size={14} />
			</button>
		{/snippet}

		{#if loadError}<p class="wbr-error pad">{loadError}</p>{/if}

		{#if loading && databases.length === 0}
			<div class="pad"><Skeleton lines={3} /></div>
		{:else if databases.length === 0}
			<EmptyState
				icon="database"
				title="No databases yet"
				description={site.isChild
					? `${site.parent} has no databases.`
					: 'Create one with the form. Leave the id empty for the domain default.'}
			/>
		{:else}
			<ul class="dbs">
				{#each databases as db (dbKey(db))}
					{@const key = dbKey(db)}
					{@const es = engineState(db.engine || 'sqlite')}
					<li>
						<div class="db-head">
							<span class="db-icon" aria-hidden="true"><Icon name="database" size={16} /></span>
							<div class="db-title">
								<strong>{key}</strong>
								{#if key === defaultId}<span class="wbr-badge neutral">default</span>{/if}
								<span class="wbr-badge neutral">{db.engine || 'sqlite'}</span>
								{#if es}<span class={['wbr-badge', es.tone]}>{es.text}</span>{/if}
							</div>
							{#if !site.isChild}
								<div class="db-actions">
									{#if db.engine === 'postgres' || db.engine === 'mariadb'}
										<button
											class="wbr-btn wbr-btn-ghost sm"
											type="button"
											disabled={busy || rotatingId === key}
											onclick={() => rotate(db)}
										>
											<Icon name="key" size={13} />
											{rotatingId === key ? 'Rotating…' : 'Rotate password'}
										</button>
									{/if}
									<button
										class="wbr-btn-danger sm"
										type="button"
										disabled={busy}
										onclick={() => confirmRemove(db)}
										aria-label={`Drop database ${key}`}
									>
										<Icon name="trash" size={13} />
									</button>
								</div>
							{/if}
						</div>
						<div class="db-fields">
							{#if db.database}<CopyField label="Database" value={db.database} />{/if}
							{#if db.username}<CopyField label="User" value={db.username} />{/if}
							{#if db.path}<CopyField label="Path" value={db.path} />{/if}
							{#if db.peer_url}<CopyField label="URL" value={db.peer_url} />{/if}
							{#if db.peer_dsn}<CopyField label="DSN" value={db.peer_dsn} />{/if}
						</div>
					</li>
				{/each}
			</ul>
		{/if}
	</Card>

	{#if !site.isChild}
		<Card title="Add database">
			<form
				class="wbr-stack"
				onsubmit={(e) => {
					e.preventDefault();
					void add();
				}}
			>
				<fieldset class="engines" disabled={busy}>
					<legend class="visually-hidden">Engine</legend>
					{#each ENGINES as opt (opt.value)}
						{@const es = engineState(opt.value)}
						<label class={['engine', engine === opt.value && 'on']}>
							<input type="radio" name="engine" value={opt.value} bind:group={engine} />
							<span class="engine-text">
								<strong>{opt.label}</strong>
								<small>{opt.hint}</small>
								{#if es}<small class={['es', es.tone]}>{es.text}</small>{/if}
							</span>
						</label>
					{/each}
				</fieldset>
				<label class="wbr-field">
					<span>Database id</span>
					<input
						name="db_id"
						bind:value={newDbId}
						placeholder={defaultId || 'default'}
						autocomplete="off"
						spellcheck="false"
						disabled={busy || blocked}
					/>
					<span class="hint">Optional. Leave empty for the domain default.</span>
				</label>
				{#if blocked}
					<div class="install" role="status">
						<span class="ic" aria-hidden="true"><Icon name="terminal" size={18} /></span>
						<div class="install-body">
							<strong>
								{needsInstall
									? `${engineLabel} is not installed`
									: `${engineLabel} is not running`}
							</strong>
							<p>
								{#if needsInstall}
									Log in as root on this VPS and install {engineLabel}. Then click Refresh and add the
									database.
								{:else}
									Log in as root and start the {engineLabel} service. Then click Refresh and add the
									database.
								{/if}
							</p>
							<pre class="cmd">{installHint || defaultCmdHint}</pre>
							<button
								class="wbr-btn wbr-btn-ghost sm"
								type="button"
								disabled={loading || busy}
								onclick={load}
							>
								<Icon name="refresh" size={14} /> Check again
							</button>
						</div>
					</div>
				{/if}
				<button class="wbr-btn" type="submit" disabled={busy || blocked}>
					<Icon name="plus" size={15} />
					{busy ? 'Working…' : 'Add database'}
				</button>
				<p class="wbr-muted small">
					Install Postgres/MariaDB as root first. Adding a database then needs step-up approval.
				</p>
			</form>
		</Card>
	{/if}
</div>

<style>
	.layout {
		display: grid;
		grid-template-columns: minmax(0, 1.6fr) minmax(17rem, 1fr);
		gap: 1rem;
		align-items: start;
	}

	.inherit {
		display: flex;
		gap: 0.6rem;
		align-items: flex-start;
		padding: 0.8rem 1rem;
		margin-bottom: 1rem;
		border-radius: var(--radius);
		background: var(--info-soft);
		color: var(--info);
	}

	.inherit p {
		margin: 0;
		color: var(--text);
		font-size: 0.9rem;
	}

	.pad {
		padding: 1rem 1.25rem;
		margin: 0;
	}

	.dbs {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.dbs li {
		padding: 1rem 1.25rem;
		border-top: 1px solid var(--border);
		display: grid;
		gap: 0.75rem;
	}

	.dbs li:first-child {
		border-top: 0;
	}

	.db-head {
		display: flex;
		align-items: center;
		gap: 0.65rem;
		flex-wrap: wrap;
	}

	.db-icon {
		display: grid;
		place-items: center;
		width: 2rem;
		height: 2rem;
		border-radius: 8px;
		background: var(--surface-3);
		color: var(--muted);
	}

	.db-title {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		flex-wrap: wrap;
		flex: 1;
		min-width: 0;
	}

	.db-title strong {
		font-family: var(--mono);
		font-size: 0.92rem;
	}

	.db-actions {
		display: flex;
		gap: 0.4rem;
	}

	.db-fields {
		display: grid;
		gap: 0.45rem;
	}

	.engines {
		border: 0;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.5rem;
	}

	.engine {
		display: flex;
		gap: 0.6rem;
		align-items: flex-start;
		padding: 0.65rem 0.75rem;
		border: 1px solid var(--border-strong);
		border-radius: var(--radius-sm);
		cursor: pointer;
	}

	.engine.on {
		border-color: var(--accent);
		background: var(--accent-soft);
	}

	.engine input {
		margin-top: 0.2rem;
		accent-color: var(--accent);
	}

	.engine-text {
		display: grid;
	}

	.engine-text strong {
		font-size: 0.88rem;
	}

	.engine-text small {
		font-size: 0.76rem;
		color: var(--muted);
	}

	.es.ok {
		color: var(--ok);
	}

	.es.warn {
		color: var(--warn);
	}

	.es.neutral {
		color: var(--muted);
	}

	.install {
		display: flex;
		gap: 0.75rem;
		align-items: flex-start;
		padding: 0.75rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-2);
	}

	.ic {
		display: grid;
		place-items: center;
		width: 2.2rem;
		height: 2.2rem;
		border-radius: 9px;
		background: var(--accent-soft);
		color: var(--accent);
		flex-shrink: 0;
	}

	.install-body {
		display: grid;
		gap: 0.55rem;
		min-width: 0;
		flex: 1;
	}

	.install-body strong {
		font-size: 0.95rem;
	}

	.install-body p {
		margin: 0;
		color: var(--muted);
		font-size: 0.88rem;
	}

	.cmd {
		margin: 0;
		padding: 0.75rem 0.9rem;
		border-radius: var(--radius-sm);
		border: 1px solid var(--border);
		background: var(--surface-3, var(--surface-2));
		font-family: var(--mono);
		font-size: 0.82rem;
		line-height: 1.45;
		overflow-x: auto;
		white-space: pre-wrap;
	}

	.small {
		font-size: 0.78rem;
		margin: 0;
	}

	@media (max-width: 1000px) {
		.layout {
			grid-template-columns: 1fr;
		}
	}
</style>
