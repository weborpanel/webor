<script lang="ts">
	import { onMount } from 'svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import FileDrop from '#lib/components/FileDrop.svelte';
	import EntryPicker from '#lib/components/EntryPicker.svelte';
	import ApiSetupWizard from '#lib/components/ApiSetupWizard.svelte';
	import ApiLogPanel from '#lib/components/ApiLogPanel.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';
	import { confirmDialog } from '#lib/stores/confirm.svelte.ts';
	import { overview } from '#lib/stores/overview.svelte.ts';

	const site = getSite();
	const MAX_BACKENDS = 8;

	type RuntimeMode = 'binary' | 'node' | 'custom';
	type ArtifactKind = 'zip' | 'binary';
	type BackendInfo = {
		enabled?: boolean;
		id?: string;
		prefix?: string;
		paths?: string[];
		websocket?: boolean;
		exec?: string;
		port?: number;
		listen?: string;
		active?: boolean;
		dir?: string;
		unit?: string;
		runtime?: { kind?: string; version?: string; command?: string };
	};

	let backends = $state.raw<BackendInfo[]>([]);
	let selectedId = $state('');
	let loading = $state(true);
	let loadError = $state('');
	let busy = $state(false);
	let saving = $state(false);
	let savingRun = $state(false);
	let deploying = $state(false);
	let showAdd = $state(false);

	let prefix = $state('/api');
	let detailLogUnit = $state('');
	let restarting = $state(false);
	let extraPaths = $state('');
	let websocket = $state(true);
	let upload = $state<File | null>(null);
	let artifactKind = $state<ArtifactKind>('binary');
	let showPicker = $state(false);

	let runtimeMode = $state<RuntimeMode>('binary');
	let nodeVersion = $state('22');
	let nodeCommand = $state('');
	let entry = $state('./server');
	let nodeChoices = $state.raw<{ id: string; label: string }[]>([]);

	let selected = $derived(backends.find((b) => b.id === selectedId) ?? null);
	let isNode = $derived(runtimeMode === 'node' || runtimeMode === 'custom');
	let atCap = $derived(backends.length >= MAX_BACKENDS);
	let publicBase = $derived.by(() => {
		const p = (prefix.trim() || '/api').replace(/\/$/, '') || '';
		return p === '' || p === '/' ? `https://${site.name}` : `https://${site.name}${p}`;
	});
	let preferExt = $derived(isNode ? ['.js', '.mjs', '.cjs'] : ([] as string[]));
	let fsListUrl = $derived(selected?.id ? site.path(`/backends/${selected.id}/fs`) : '');
	let selectedDir = $derived(selected?.dir || (selected?.id ? `apis/${selected.id}` : 'apis/…'));
	let detailLogOptions = $derived.by(() => {
		if (!selected?.id) return [];
		const opts = [{ value: `api:${selected.id}`, label: 'This API' }];
		if (backends.length > 1) opts.push({ value: 'apis', label: 'All APIs' });
		return opts;
	});

	function defaultEntry(mode: RuntimeMode): string {
		return mode === 'binary' ? './server' : 'server.js';
	}

	function defaultArtifact(mode: RuntimeMode): ArtifactKind {
		return mode === 'binary' ? 'binary' : 'zip';
	}

	function runtimeBadge(b: BackendInfo): string {
		const r = b.runtime;
		if (r?.command) return 'custom';
		if (r?.kind === 'node') return 'node';
		return 'binary';
	}

	function parseExtraPaths(raw: string): string[] {
		return raw
			.split(',')
			.map((p) => p.trim())
			.filter(Boolean);
	}

	function syncForm(b: BackendInfo | null) {
		showPicker = false;
		upload = null;
		if (!b?.enabled) {
			prefix = '/api';
			extraPaths = '';
			websocket = true;
			runtimeMode = 'binary';
			entry = './server';
			nodeCommand = '';
			nodeVersion = '22';
			artifactKind = 'binary';
			return;
		}
		prefix = b.prefix || '/api';
		const primary = (b.prefix || '/api').replace(/\/$/, '') || '/api';
		extraPaths = (b.paths || [])
			.filter((p) => {
				const n = p.replace(/\/$/, '') || '/';
				return n !== primary;
			})
			.join(', ');
		websocket = b.websocket ?? true;
		const r = b.runtime;
		if (r?.command) {
			runtimeMode = 'custom';
			nodeCommand = r.command;
		} else if (r?.kind === 'node') {
			runtimeMode = 'node';
			nodeVersion = r.version || '22';
		} else {
			runtimeMode = 'binary';
		}
		entry = b.exec || defaultEntry(runtimeMode);
		artifactKind = defaultArtifact(runtimeMode);
	}

	function selectBackend(id: string) {
		selectedId = id;
		const b = backends.find((x) => x.id === id) ?? null;
		syncForm(b);
		detailLogUnit = id ? `api:${id}` : '';
	}

	function onRuntimeModeChange(mode: RuntimeMode) {
		runtimeMode = mode;
		entry = defaultEntry(mode);
		artifactKind = defaultArtifact(mode);
	}

	async function load() {
		loading = true;
		try {
			const [br, vers] = await Promise.all([
				request<{ backends?: BackendInfo[] }>(site.path('/backends')),
				request<{ versions?: { id: string; label: string }[] }>('/api/runtimes/node')
			]);
			if (vers.ok) nodeChoices = vers.data.versions ?? [];
			if (!br.ok) {
				loadError = br.data.error || 'Could not load APIs';
				return;
			}
			loadError = '';
			backends = (br.data.backends ?? []).filter((b) => b.enabled !== false && b.id);
			if (backends.length === 0) {
				selectedId = '';
				syncForm(null);
				showAdd = true;
			} else {
				const keep = backends.find((b) => b.id === selectedId);
				selectBackend(keep?.id || backends[0].id || '');
				showAdd = false;
			}
		} catch (e) {
			loadError = errorMessage(e);
		} finally {
			loading = false;
		}
	}

	async function onWizardDone(id: string) {
		showAdd = false;
		await load();
		selectBackend(id);
		void overview.refresh();
	}

	async function restartSelected() {
		if (!selected?.id) return;
		restarting = true;
		try {
			const r = await request<{ backend?: BackendInfo }>(
				site.path(`/backends/${selected.id}/restart`),
				{ method: 'POST' }
			);
			if (!r.ok) {
				toasts.error(r.data.error || 'Restart failed');
				return;
			}
			toasts.success(r.data.backend?.active ? 'API running' : 'Restart requested');
			await load();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			restarting = false;
		}
	}

	async function saveRuntime(e: SubmitEvent) {
		e.preventDefault();
		if (!selected?.id) return;
		savingRun = true;
		try {
			const payload: Record<string, unknown> = { exec: entry.trim() };
			if (runtimeMode === 'binary') {
				payload.runtime = { clear: true, kind: '' };
			} else if (runtimeMode === 'custom') {
				payload.runtime = { kind: 'node', command: nodeCommand.trim() };
			} else {
				payload.runtime = { kind: 'node', version: nodeVersion };
			}
			const r = await request<{ backend?: BackendInfo }>(site.path(`/backends/${selected.id}`), {
				method: 'PATCH',
				json: payload
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not save runtime');
				return;
			}
			toasts.success('Runtime saved');
			await load();
			void overview.refresh();
		} catch (err) {
			toasts.error(errorMessage(err));
		} finally {
			savingRun = false;
		}
	}

	async function saveSettings(e: SubmitEvent) {
		e.preventDefault();
		if (!selected?.id) return;
		saving = true;
		try {
			const r = await request<{ backend?: BackendInfo }>(site.path(`/backends/${selected.id}`), {
				method: 'PATCH',
				json: {
					prefix: prefix.trim() || '/api',
					paths: parseExtraPaths(extraPaths),
					websocket
				}
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not save settings');
				return;
			}
			toasts.success('Settings saved');
			await load();
		} catch (err) {
			toasts.error(errorMessage(err));
		} finally {
			saving = false;
		}
	}

	async function deployUpload(e: SubmitEvent) {
		e.preventDefault();
		if (!selected?.id) return;
		if (!upload) {
			toasts.error(artifactKind === 'zip' ? 'Choose a zip file' : 'Choose a binary file');
			return;
		}
		deploying = true;
		try {
			const body = new FormData();
			body.append('file', upload);
			const path =
				artifactKind === 'zip'
					? site.path(`/backends/${selected.id}/deploy-zip`)
					: site.path(`/backends/${selected.id}/deploy`);
			const r = await request<{ release_id?: string; active?: boolean; root?: string }>(path, {
				method: 'POST',
				body
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Deploy failed');
				return;
			}
			const usedZip = artifactKind === 'zip';
			toasts.success(
				usedZip
					? `Zip extracted to ${selectedDir}/`
					: r.data.release_id
						? `Deployed release ${r.data.release_id}`
						: `Binary installed as ${selectedDir}/server`
			);
			upload = null;
			await load();
			if (usedZip) showPicker = true;
			void overview.refresh();
		} catch (err) {
			toasts.error(errorMessage(err));
		} finally {
			deploying = false;
		}
	}

	async function detach() {
		if (!selected?.id) return;
		const id = selected.id;
		const label = selected.prefix || '/api';
		const ok = await confirmDialog({
			title: 'Detach API?',
			message: `Remove the process and path proxy for ${label} on ${site.name}. Files under ${selectedDir}/ stay on disk.`,
			confirmLabel: 'Detach',
			danger: true,
			typeToConfirm: label
		});
		if (!ok) return;
		busy = true;
		try {
			const r = await request(site.path(`/backends/${id}`), { method: 'DELETE' });
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not detach');
				return;
			}
			toasts.success('API detached');
			upload = null;
			selectedId = '';
			await load();
			void overview.refresh();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			busy = false;
		}
	}

	onMount(() => {
		if (!site.isApp) void load();
		else loading = false;
	});
</script>

{#if site.isApp}
	<EmptyState
		icon="server"
		title="API backends attach to static sites"
		description="Use Deploy for app sites."
	>
		<a class="wbr-btn" href={site.href('/deploy')}>Go to Deploy</a>
	</EmptyState>
{:else}
	<div class="layout">
		<Card
			title="APIs"
			description={`Run Linux binaries or Node apps behind path prefixes. Each API is its own process. Webor injects PORT and proxies traffic. At most ${MAX_BACKENDS} per site.`}
		>
			{#snippet actions()}
				{#if backends.length > 0 && !showAdd && !atCap}
					<button
						type="button"
						class="wbr-btn sm"
						disabled={loading || busy}
						onclick={() => (showAdd = true)}
					>
						<Icon name="plus" size={14} />
						Add API
					</button>
				{/if}
				<button
					type="button"
					class="wbr-btn wbr-btn-ghost sm"
					disabled={loading || busy}
					onclick={load}
					aria-label="Refresh APIs"
				>
					<Icon name="refresh" size={14} />
				</button>
			{/snippet}

			{#if loadError}<p class="wbr-error">{loadError}</p>{/if}

			{#if loading && backends.length === 0}
				<Skeleton lines={4} />
			{:else if showAdd || backends.length === 0}
				{#if backends.length > 0}
					<button
						type="button"
						class="wbr-btn wbr-btn-ghost sm back-list"
						disabled={busy}
						onclick={() => (showAdd = false)}
					>
						<Icon name="chevronLeft" size={14} /> Back to list
					</button>
				{/if}
				<ApiSetupWizard
					{nodeChoices}
					ondone={(id) => void onWizardDone(id)}
					oncancel={() => {
						showAdd = false;
						void load();
					}}
				/>
			{:else}
				<ul class="api-list">
					{#each backends as b (b.id)}
						<li>
							<button
								type="button"
								class={['api-row', selectedId === b.id && 'on']}
								onclick={() => selectBackend(b.id || '')}
							>
								<span class="ic" aria-hidden="true"><Icon name="server" size={16} /></span>
								<span class="api-meta">
									<strong>{b.prefix || '/api'}</strong>
									<span class="wbr-badge neutral">{runtimeBadge(b)}</span>
									{#if b.active}
										<span class="wbr-badge ok">running</span>
									{:else}
										<span class="wbr-badge warn">stopped</span>
									{/if}
								</span>
							</button>
						</li>
					{/each}
				</ul>

				{#if selected}
					<div class="detail">
						<div class="head">
							<span class="ic" aria-hidden="true"><Icon name="server" size={18} /></span>
							<div class="title">
								<strong>{selected.prefix || '/api'}</strong>
								{#if selected.active}
									<span class="wbr-badge ok">running</span>
								{:else}
									<span class="wbr-badge warn">stopped</span>
								{/if}
								<span class="wbr-badge neutral">{runtimeBadge(selected)}</span>
							</div>
							<div class="acts">
								<button
									class="wbr-btn wbr-btn-ghost sm"
									type="button"
									disabled={busy || saving || savingRun || deploying || restarting}
									onclick={() => void restartSelected()}
								>
									<Icon name="refresh" size={13} />
									{restarting ? 'Restarting…' : 'Restart'}
								</button>
								<button
									class="wbr-btn-danger sm"
									type="button"
									disabled={busy || saving || savingRun || deploying || restarting}
									onclick={detach}
								>
									<Icon name="trash" size={13} /> Detach
								</button>
							</div>
						</div>

						<div class="env-box" role="note">
							{#if isNode}
								<p>
									Listen on <code>process.env.PORT</code> (and optional <code>HOST</code>, default
									<code>127.0.0.1</code>). Working directory is
									<code>{selectedDir}/</code>. Webor sets <code>PORT</code> /
									<code>WBR_PORT</code> — you never pick a port.
								</p>
								<pre class="snippet">const port = process.env.PORT || 3000;</pre>
							{:else}
								<p>
									Bind to <code>PORT</code> / <code>HOST</code> from the environment. Working directory
									is <code>{selectedDir}/</code>. Webor sets <code>PORT</code> /
									<code>WBR_PORT</code> on the service — you never pick a port.
								</p>
								<pre class="snippet">HOST=127.0.0.1 PORT=$PORT ./server</pre>
							{/if}
						</div>

						<form class="runtime" onsubmit={saveRuntime}>
							<h3>How it runs</h3>
							<label class="wbr-field">
								<span>Interpreter</span>
								<select
									value={runtimeMode}
									disabled={savingRun || busy}
									onchange={(e) => onRuntimeModeChange(e.currentTarget.value as RuntimeMode)}
								>
									<option value="binary">Binary (raw ExecStart)</option>
									<option value="node">Node (managed)</option>
									<option value="custom">Node (custom path)</option>
								</select>
							</label>
							{#if runtimeMode === 'node'}
								<label class="wbr-field">
									<span>Node version</span>
									<select bind:value={nodeVersion} disabled={savingRun || busy}>
										{#each nodeChoices as v (v.id)}
											<option value={v.id}>{v.label}</option>
										{:else}
											<option value="20">Node 20</option>
											<option value="22">Node 22</option>
											<option value="24">Node 24</option>
										{/each}
									</select>
								</label>
							{/if}
							{#if runtimeMode === 'custom'}
								<label class="wbr-field">
									<span>Absolute path to node</span>
									<input
										bind:value={nodeCommand}
										placeholder="/usr/local/bin/node"
										disabled={savingRun || busy}
										spellcheck="false"
									/>
								</label>
							{/if}
							<label class="wbr-field">
								<span>{isNode ? 'Entry script' : 'Binary path'}</span>
								<div class="entry-row">
									<input
										bind:value={entry}
										placeholder={isNode ? 'server.js' : './server'}
										disabled={savingRun || busy}
										spellcheck="false"
									/>
									<button
										type="button"
										class="wbr-btn wbr-btn-ghost sm"
										disabled={savingRun || busy || !selected.id}
										onclick={() => (showPicker = !showPicker)}
									>
										<Icon name="search" size={14} />
										{showPicker ? 'Hide' : 'Browse'}
									</button>
								</div>
								{#if showPicker && fsListUrl}
									{#key fsListUrl}
										<EntryPicker
											listUrl={fsListUrl}
											value={entry}
											{preferExt}
											disabled={savingRun || busy}
											onselect={(path) => {
												entry = path;
											}}
											onclose={() => (showPicker = false)}
										/>
									{/key}
								{/if}
								<span class="hint">
									Relative to <code>{selectedDir}/</code>. Default
									<code>{defaultEntry(runtimeMode)}</code>.
								</span>
							</label>
							<button class="wbr-btn" type="submit" disabled={savingRun || busy}>
								<Icon name="check" size={15} />
								{savingRun ? 'Saving…' : 'Save runtime'}
							</button>
						</form>

						<form class="deploy" onsubmit={deployUpload}>
							<h3>Deploy</h3>
							<p class="deploy-hint">
								Zip extracts into <code>{selectedDir}/</code>; binary installs as
								<code>{selectedDir}/server</code>. The Files tab is <code>public/</code> only —
								API artifacts are not there. Public URL: <code>{publicBase}/…</code>
							</p>
							<div class="seg" role="group" aria-label="Artifact type">
								<button
									type="button"
									class={['seg-btn', artifactKind === 'zip' && 'on']}
									disabled={deploying || busy}
									onclick={() => (artifactKind = 'zip')}
								>
									Zip
								</button>
								<button
									type="button"
									class={['seg-btn', artifactKind === 'binary' && 'on']}
									disabled={deploying || busy}
									onclick={() => (artifactKind = 'binary')}
								>
									Binary
								</button>
							</div>
							<FileDrop
								bind:file={upload}
								label={artifactKind === 'zip' ? 'Choose a zip' : 'Choose a binary'}
								accept={artifactKind === 'zip' ? '.zip,application/zip' : undefined}
								disabled={deploying || busy}
							/>
							<button class="wbr-btn" type="submit" disabled={deploying || busy || !upload}>
								<Icon name="upload" size={15} />
								{deploying ? 'Deploying…' : 'Deploy'}
							</button>
						</form>

						<form class="settings" onsubmit={saveSettings}>
							<h3>Proxy settings</h3>
							<label class="wbr-field">
								<span>Path prefix</span>
								<input
									bind:value={prefix}
									placeholder="/api"
									disabled={saving || busy}
									spellcheck="false"
								/>
								<span class="hint">
									Use <code>/</code> to proxy the whole host (typical for
									<code>api.…</code> subdomains), or <code>/api</code> for path-based APIs.
								</span>
							</label>
							<label class="wbr-field">
								<span>Extra paths (comma-separated)</span>
								<input
									bind:value={extraPaths}
									placeholder="/ws"
									disabled={saving || busy}
									spellcheck="false"
								/>
								<span class="hint">Additional path prefixes proxied to the same process.</span>
							</label>
							<label class="toggle">
								<input type="checkbox" bind:checked={websocket} disabled={saving || busy} />
								<span>
									<strong>WebSocket support</strong>
									<small>Allow WebSocket upgrades through these paths. No Caddy editing needed.</small>
								</span>
							</label>
							<button class="wbr-btn" type="submit" disabled={saving || busy}>
								<Icon name="check" size={15} />
								{saving ? 'Saving…' : 'Save settings'}
							</button>
						</form>

						{#if detailLogUnit}
							<section class="logs-sec">
								<h3>Logs</h3>
								{#if selected.unit}
									<p class="svc-line">Service <code>{selected.unit}</code></p>
								{/if}
								{#key detailLogUnit}
									<ApiLogPanel
										bind:unit={detailLogUnit}
										unitOptions={detailLogOptions}
										compact
										follow={true}
									/>
								{/key}
							</section>
						{/if}
					</div>
				{/if}
			{/if}
		</Card>

		<Card title="Connecting">
			<ul class="tips">
				<li>
					Use <strong>Add API</strong> for the guided wizard: path → upload → entry → start with live
					logs. You can go back to fix errors until the process stays up.
				</li>
				<li>
					Each API has its own folder under <code>apis/</code>. Read <code>PORT</code> /
					<code>HOST</code> from the environment — Webor injects them.
				</li>
				<li>
					The Logs tab can show one API or <strong>All APIs</strong>. Files is <code>public/</code> only.
				</li>
			</ul>
		</Card>
	</div>
{/if}

<style>
	.layout {
		display: grid;
		grid-template-columns: minmax(0, 1.6fr) minmax(16rem, 1fr);
		gap: 1rem;
		align-items: start;
	}

	.api-list {
		list-style: none;
		margin: 0 0 1rem;
		padding: 0;
		display: grid;
		gap: 0.35rem;
	}

	.api-row {
		display: flex;
		align-items: center;
		gap: 0.65rem;
		width: 100%;
		padding: 0.55rem 0.65rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: transparent;
		color: inherit;
		font: inherit;
		text-align: left;
		cursor: pointer;
	}

	.api-row:hover {
		background: var(--surface-2, var(--bg-elevated, var(--panel)));
	}

	.api-row.on {
		border-color: var(--accent);
		background: var(--accent-soft);
	}

	.api-meta {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.35rem;
		min-width: 0;
	}

	.api-meta strong {
		font-family: var(--mono);
	}

	.detail {
		margin-top: 0.25rem;
	}

	.env-box {
		margin-bottom: 1rem;
		padding: 0.85rem 1rem;
		border-radius: var(--radius);
		background: var(--surface-2, var(--bg-elevated, var(--panel)));
		border: 1px solid var(--border);
	}

	.env-box p {
		margin: 0 0 0.55rem;
		font-size: 0.88rem;
		color: var(--muted);
		line-height: 1.45;
	}

	.snippet {
		margin: 0;
		padding: 0.55rem 0.7rem;
		border-radius: calc(var(--radius) - 2px);
		background: var(--bg, #0f1115);
		color: var(--text);
		font-family: var(--mono);
		font-size: 0.8rem;
		overflow-x: auto;
	}

	.head {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.65rem;
		margin-bottom: 0.9rem;
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

	.api-row .ic {
		width: 1.85rem;
		height: 1.85rem;
		border-radius: 7px;
	}

	.title {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.4rem;
		flex: 1;
	}

	.title strong {
		font-family: var(--mono);
	}

	.acts {
		display: flex;
		gap: 0.4rem;
	}

	.runtime,
	.settings,
	.deploy {
		display: grid;
		gap: 0.75rem;
		margin-top: 1.25rem;
		padding-top: 1.1rem;
		border-top: 1px solid var(--border);
	}

	.runtime {
		margin-top: 0.5rem;
		padding-top: 0;
		border-top: none;
	}

	.runtime h3,
	.settings h3,
	.deploy h3 {
		margin: 0;
		font-size: 0.95rem;
	}

	.entry-row {
		display: flex;
		gap: 0.45rem;
		align-items: center;
	}

	.entry-row input {
		flex: 1;
		min-width: 0;
	}

	.deploy-hint {
		margin: 0;
		font-size: 0.86rem;
		color: var(--muted);
		line-height: 1.45;
	}

	.logs-sec {
		margin-top: 1.25rem;
		padding-top: 1.1rem;
		border-top: 1px solid var(--border);
		display: grid;
		gap: 0.65rem;
	}

	.logs-sec h3 {
		margin: 0;
		font-size: 0.95rem;
	}

	.svc-line {
		margin: 0;
		font-size: 0.82rem;
		color: var(--muted);
	}

	.back-list {
		margin-bottom: 0.75rem;
	}

	.seg {
		display: inline-flex;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		overflow: hidden;
		width: fit-content;
	}

	.seg-btn {
		padding: 0.4rem 0.85rem;
		border: 0;
		background: transparent;
		color: var(--muted);
		font: inherit;
		font-size: 0.86rem;
		cursor: pointer;
	}

	.seg-btn + .seg-btn {
		border-left: 1px solid var(--border);
	}

	.seg-btn.on {
		background: var(--accent-soft);
		color: var(--accent);
		font-weight: 600;
	}

	.toggle {
		display: flex;
		align-items: flex-start;
		gap: 0.55rem;
		font-size: 0.88rem;
		color: var(--text);
	}

	.toggle input {
		margin-top: 0.2rem;
	}

	.toggle span {
		display: grid;
		gap: 0.15rem;
	}

	.toggle strong {
		font-size: 0.88rem;
	}

	.toggle small {
		color: var(--muted);
		font-size: 0.8rem;
		font-weight: 400;
	}

	.tips {
		margin: 0;
		padding-left: 1.1rem;
		display: grid;
		gap: 0.5rem;
		font-size: 0.88rem;
		color: var(--muted);
	}

	@media (max-width: 1000px) {
		.layout {
			grid-template-columns: 1fr;
		}
	}
</style>
