<script lang="ts">
	import Icon from './Icon.svelte';
	import FileDrop from './FileDrop.svelte';
	import EntryPicker from './EntryPicker.svelte';
	import ApiLogPanel from './ApiLogPanel.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';

	type RuntimeMode = 'binary' | 'node' | 'custom';
	type ArtifactKind = 'zip' | 'binary';
	type BackendInfo = {
		id?: string;
		prefix?: string;
		unit?: string;
		dir?: string;
		active?: boolean;
		exec?: string;
		runtime?: { kind?: string; version?: string; command?: string };
	};

	type Props = {
		nodeChoices?: { id: string; label: string }[];
		ondone?: (backendId: string) => void;
		oncancel?: () => void;
	};

	let { nodeChoices = [], ondone, oncancel }: Props = $props();

	const site = getSite();
	const steps = ['Path', 'Upload', 'Run', 'Start'] as const;

	let step = $state(0);
	let busy = $state(false);
	let backend = $state.raw<BackendInfo | null>(null);

	// Step 1
	let prefix = $state('/api');
	let extraPaths = $state('');
	let websocket = $state(true);

	// Step 2
	let artifactKind = $state<ArtifactKind>('zip');
	let upload = $state<File | null>(null);
	let uploaded = $state(false);

	// Step 3
	let runtimeMode = $state<RuntimeMode>('node');
	let nodeVersion = $state('22');
	let nodeCommand = $state('');
	let entry = $state('server.js');
	let extraParams = $state('');
	let showPicker = $state(true);

	// Step 4
	let started = $state(false);
	let logUnit = $state('');

	let backendId = $derived(backend?.id || '');
	let unitName = $derived(backend?.unit || (backendId ? `webor-backend-…-${backendId}.service` : ''));
	let fsListUrl = $derived(backendId ? site.path(`/backends/${backendId}/fs`) : '');
	let isNode = $derived(runtimeMode === 'node' || runtimeMode === 'custom');
	let preferExt = $derived(isNode ? ['.js', '.mjs', '.cjs'] : ([] as string[]));
	let logOptions = $derived.by(() => {
		const opts = [{ value: `api:${backendId}`, label: 'This API' }];
		opts.push({ value: 'apis', label: 'All APIs' });
		return opts;
	});

	function parseExtraPaths(raw: string): string[] {
		return raw
			.split(',')
			.map((p) => p.trim())
			.filter(Boolean);
	}

	function buildExec(): string {
		const e = entry.trim();
		const p = extraParams.trim();
		return p ? `${e} ${p}` : e;
	}

	function inferRuntimeFromEntry(path: string) {
		const lower = path.toLowerCase();
		if (/\.(js|mjs|cjs)$/.test(lower)) {
			if (runtimeMode === 'binary') {
				runtimeMode = 'node';
				artifactKind = 'zip';
			}
		}
	}

	async function createBackend() {
		busy = true;
		try {
			const r = await request<{ backend?: BackendInfo }>(site.path('/backends'), {
				method: 'POST',
				json: {
					prefix: prefix.trim() || '/api',
					paths: parseExtraPaths(extraPaths),
					websocket
				}
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not create API');
				return false;
			}
			backend = r.data.backend ?? null;
			if (backend?.id) logUnit = `api:${backend.id}`;
			return !!backend?.id;
		} catch (e) {
			toasts.error(errorMessage(e));
			return false;
		} finally {
			busy = false;
		}
	}

	async function deployArtifact() {
		if (!backendId || !upload) {
			toasts.error(artifactKind === 'zip' ? 'Choose a zip file' : 'Choose a binary file');
			return false;
		}
		busy = true;
		try {
			const body = new FormData();
			body.append('file', upload);
			const path =
				artifactKind === 'zip'
					? site.path(`/backends/${backendId}/deploy-zip`)
					: site.path(`/backends/${backendId}/deploy`);
			const r = await request(path, { method: 'POST', body });
			if (!r.ok) {
				toasts.error(r.data.error || 'Upload failed');
				return false;
			}
			uploaded = true;
			if (artifactKind === 'binary') {
				runtimeMode = 'binary';
				entry = './server';
			} else if (runtimeMode === 'binary') {
				runtimeMode = 'node';
				entry = 'server.js';
			}
			showPicker = true;
			toasts.success(artifactKind === 'zip' ? 'Zip extracted' : 'Binary installed');
			return true;
		} catch (e) {
			toasts.error(errorMessage(e));
			return false;
		} finally {
			busy = false;
		}
	}

	async function saveRuntime() {
		if (!backendId) return false;
		if (!entry.trim()) {
			toasts.error('Choose an entry file');
			return false;
		}
		busy = true;
		try {
			const payload: Record<string, unknown> = { exec: buildExec() };
			if (runtimeMode === 'binary') {
				payload.runtime = { clear: true, kind: '' };
			} else if (runtimeMode === 'custom') {
				payload.runtime = { kind: 'node', command: nodeCommand.trim() };
			} else {
				payload.runtime = { kind: 'node', version: nodeVersion };
			}
			const r = await request<{ backend?: BackendInfo }>(site.path(`/backends/${backendId}`), {
				method: 'PATCH',
				json: payload
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not save runtime');
				return false;
			}
			backend = r.data.backend ?? backend;
			toasts.success('Runtime saved');
			return true;
		} catch (e) {
			toasts.error(errorMessage(e));
			return false;
		} finally {
			busy = false;
		}
	}

	async function startApi() {
		if (!backendId) return;
		busy = true;
		try {
			// Ensure latest exec/runtime before restart
			if (!(await saveRuntime())) return;
			const r = await request<{ backend?: BackendInfo }>(
				site.path(`/backends/${backendId}/restart`),
				{ method: 'POST' }
			);
			if (!r.ok) {
				toasts.error(r.data.error || 'Start failed — check the logs below');
				started = true;
				return;
			}
			backend = r.data.backend ?? backend;
			started = true;
			logUnit = `api:${backendId}`;
			toasts.success(backend?.active ? 'API running' : 'Start requested — check logs');
		} catch (e) {
			toasts.error(errorMessage(e));
			started = true;
		} finally {
			busy = false;
		}
	}

	async function next() {
		if (step === 0) {
			if (!backendId) {
				const ok = await createBackend();
				if (!ok) return;
			} else {
				// Update path if they went back
				busy = true;
				try {
					const r = await request<{ backend?: BackendInfo }>(site.path(`/backends/${backendId}`), {
						method: 'PATCH',
						json: {
							prefix: prefix.trim() || '/api',
							paths: parseExtraPaths(extraPaths),
							websocket
						}
					});
					if (!r.ok) {
						toasts.error(r.data.error || 'Could not update path');
						return;
					}
					backend = r.data.backend ?? backend;
				} catch (e) {
					toasts.error(errorMessage(e));
					return;
				} finally {
					busy = false;
				}
			}
			step = 1;
			return;
		}
		if (step === 1) {
			if (!uploaded) {
				const ok = await deployArtifact();
				if (!ok) return;
			}
			step = 2;
			return;
		}
		if (step === 2) {
			const ok = await saveRuntime();
			if (!ok) return;
			step = 3;
			return;
		}
	}

	function back() {
		if (step > 0) step -= 1;
	}

	async function cancelWizard() {
		if (backendId && !started) {
			// Best-effort cleanup of unused backend created mid-wizard
			try {
				await request(site.path(`/backends/${backendId}`), { method: 'DELETE' });
			} catch {
				/* ignore */
			}
		}
		oncancel?.();
	}
</script>

<div class="wizard">
	<ol class="steps" aria-label="Setup steps">
		{#each steps as label, i (label)}
			<li class={['step', i === step && 'on', i < step && 'done']}>
				<span class="num">{i + 1}</span>
				<span class="lbl">{label}</span>
			</li>
		{/each}
	</ol>

	{#if step === 0}
		<div class="pane">
			<h3>Path</h3>
			<p class="hint">Choose the URL path Caddy will proxy to this process.</p>
			<label class="wbr-field">
				<span>Path prefix</span>
				<input bind:value={prefix} placeholder="/api" disabled={busy} spellcheck="false" />
				<span class="hint">Use <code>/</code> for the whole host, or <code>/api</code>, <code>/ssr</code>, etc.</span>
			</label>
			<label class="wbr-field">
				<span>Extra paths (optional)</span>
				<input bind:value={extraPaths} placeholder="/ws" disabled={busy} spellcheck="false" />
			</label>
			<label class="toggle">
				<input type="checkbox" bind:checked={websocket} disabled={busy} />
				<span>WebSocket support</span>
			</label>
			{#if unitName && backendId}
				<p class="svc">Service will be <code>{unitName}</code></p>
			{/if}
		</div>
	{:else if step === 1}
		<div class="pane">
			<h3>Upload</h3>
			<p class="hint">
				Zip extracts into <code>{backend?.dir || 'apis/…'}/</code>. Binary installs as
				<code>{backend?.dir || 'apis/…'}/server</code>.
			</p>
			{#if unitName}
				<p class="svc">Service: <code>{unitName}</code></p>
			{/if}
			<div class="seg" role="group" aria-label="Artifact type">
				<button
					type="button"
					class={['seg-btn', artifactKind === 'zip' && 'on']}
					disabled={busy}
					onclick={() => {
						artifactKind = 'zip';
						uploaded = false;
					}}
				>
					Zip
				</button>
				<button
					type="button"
					class={['seg-btn', artifactKind === 'binary' && 'on']}
					disabled={busy}
					onclick={() => {
						artifactKind = 'binary';
						uploaded = false;
					}}
				>
					Binary
				</button>
			</div>
			<FileDrop
				bind:file={upload}
				label={artifactKind === 'zip' ? 'Choose a zip' : 'Choose a binary'}
				accept={artifactKind === 'zip' ? '.zip,application/zip' : undefined}
				disabled={busy}
				onselect={() => (uploaded = false)}
			/>
			{#if uploaded}
				<p class="ok">Upload complete — continue to pick the entry file.</p>
			{/if}
		</div>
	{:else if step === 2}
		<div class="pane">
			<h3>How it runs</h3>
			{#if unitName}
				<p class="svc">Service: <code>{unitName}</code></p>
			{/if}
			<label class="wbr-field">
				<span>Interpreter</span>
				<select bind:value={runtimeMode} disabled={busy}>
					<option value="binary">Binary</option>
					<option value="node">Node (managed)</option>
					<option value="custom">Node (custom path)</option>
				</select>
			</label>
			{#if runtimeMode === 'node'}
				<label class="wbr-field">
					<span>Node version</span>
					<select bind:value={nodeVersion} disabled={busy}>
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
					<input bind:value={nodeCommand} placeholder="/usr/local/bin/node" disabled={busy} spellcheck="false" />
				</label>
			{/if}
			<label class="wbr-field">
				<span>{isNode ? 'Entry script' : 'Binary path'}</span>
				<input bind:value={entry} placeholder={isNode ? 'server.js' : './server'} disabled={busy} spellcheck="false" />
			</label>
			{#if fsListUrl}
				<button type="button" class="wbr-btn wbr-btn-ghost sm" disabled={busy} onclick={() => (showPicker = !showPicker)}>
					<Icon name="search" size={14} />
					{showPicker ? 'Hide files' : 'Browse files'}
				</button>
				{#if showPicker}
					{#key fsListUrl}
						<EntryPicker
							listUrl={fsListUrl}
							value={entry}
							{preferExt}
							disabled={busy}
							onselect={(path) => {
								entry = path;
								inferRuntimeFromEntry(path);
							}}
						/>
					{/key}
				{/if}
			{/if}
			<label class="wbr-field">
				<span>Extra params (optional)</span>
				<input
					bind:value={extraParams}
					placeholder="--flag value"
					disabled={busy}
					spellcheck="false"
				/>
				<span class="hint">Appended after the entry. <code>PORT</code> / <code>HOST</code> are already set by Webor.</span>
			</label>
			<p class="exec-preview">
				ExecStart preview: <code>{isNode ? `node ${buildExec()}` : buildExec()}</code>
			</p>
		</div>
	{:else}
		<div class="pane">
			<h3>Start</h3>
			{#if unitName}
				<p class="svc">Service: <code>{unitName}</code></p>
			{/if}
			<p class="hint">
				Start the unit and watch the journal. If it fails, go <strong>Back</strong>, fix the entry or
				re-upload, then start again.
			</p>
			<button class="wbr-btn" type="button" disabled={busy || !backendId} onclick={() => void startApi()}>
				<Icon name="rocket" size={15} />
				{busy ? 'Starting…' : started ? 'Restart API' : 'Start API'}
			</button>
			{#if started && backendId && logUnit}
				<div class="logs">
					<ApiLogPanel bind:unit={logUnit} unitOptions={logOptions} compact follow={true} />
				</div>
			{/if}
		</div>
	{/if}

	<div class="nav">
		{#if step === 0}
			<button type="button" class="wbr-btn wbr-btn-ghost" disabled={busy} onclick={() => void cancelWizard()}>
				Cancel
			</button>
		{:else}
			<button type="button" class="wbr-btn wbr-btn-ghost" disabled={busy} onclick={back}>
				<Icon name="chevronLeft" size={14} /> Back
			</button>
		{/if}
		<div class="spacer"></div>
		{#if step < 3}
			<button type="button" class="wbr-btn" disabled={busy} onclick={() => void next()}>
				{busy ? 'Working…' : 'Next'}
				<Icon name="chevronRight" size={14} />
			</button>
		{:else}
			<button
				type="button"
				class="wbr-btn"
				disabled={busy || !backendId}
				onclick={() => backendId && ondone?.(backendId)}
			>
				<Icon name="check" size={15} />
				Done
			</button>
		{/if}
	</div>
</div>

<style>
	.wizard {
		display: grid;
		gap: 1rem;
	}

	.steps {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}

	.step {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		padding: 0.35rem 0.65rem;
		border-radius: 999px;
		border: 1px solid var(--border);
		font-size: 0.82rem;
		color: var(--muted);
	}

	.step.on {
		border-color: var(--accent);
		color: var(--accent);
		background: var(--accent-soft);
		font-weight: 600;
	}

	.step.done {
		color: var(--ok, #2a7);
	}

	.num {
		display: grid;
		place-items: center;
		width: 1.25rem;
		height: 1.25rem;
		border-radius: 50%;
		background: var(--surface-2);
		font-size: 0.75rem;
		font-weight: 700;
	}

	.step.on .num {
		background: var(--accent);
		color: #fff;
	}

	.pane {
		display: grid;
		gap: 0.75rem;
	}

	.pane h3 {
		margin: 0;
		font-size: 1rem;
	}

	.hint {
		margin: 0;
		font-size: 0.86rem;
		color: var(--muted);
		line-height: 1.45;
	}

	.svc,
	.exec-preview,
	.ok {
		margin: 0;
		font-size: 0.86rem;
	}

	.ok {
		color: var(--ok, #2a7);
	}

	.toggle {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.88rem;
	}

	.seg {
		display: inline-flex;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		overflow: hidden;
		width: fit-content;
	}

	.seg-btn {
		border: 0;
		background: transparent;
		padding: 0.4rem 0.9rem;
		font: inherit;
		font-size: 0.86rem;
		cursor: pointer;
		color: var(--muted);
	}

	.seg-btn + .seg-btn {
		border-left: 1px solid var(--border);
	}

	.seg-btn.on {
		background: var(--accent-soft);
		color: var(--accent);
		font-weight: 600;
	}

	.logs {
		margin-top: 0.5rem;
	}

	.nav {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding-top: 0.5rem;
		border-top: 1px solid var(--border);
	}

	.spacer {
		flex: 1;
	}
</style>
