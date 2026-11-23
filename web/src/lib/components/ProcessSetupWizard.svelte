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
	type ProcessInfo = {
		name?: string;
		exec?: string;
		dir?: string;
		unit?: string;
		active?: boolean;
		runtime?: { kind?: string; version?: string; command?: string };
	};

	type Props = {
		nodeChoices?: { id: string; label: string }[];
		ondone?: (jobName: string) => void;
		oncancel?: () => void;
	};

	let { nodeChoices = [], ondone, oncancel }: Props = $props();

	const site = getSite();
	const steps = ['Name', 'Upload', 'Run', 'Start'] as const;

	let step = $state(0);
	let busy = $state(false);
	let proc = $state.raw<ProcessInfo | null>(null);

	let jobName = $state('');
	let artifactKind = $state<ArtifactKind>('zip');
	let upload = $state<File | null>(null);
	let uploaded = $state(false);

	let runtimeMode = $state<RuntimeMode>('node');
	let nodeVersion = $state('22');
	let nodeCommand = $state('');
	let entry = $state('server.js');
	let extraParams = $state('');
	let showPicker = $state(true);

	let started = $state(false);
	let logUnit = $state('');

	let jobId = $derived(proc?.name || '');
	let fsListUrl = $derived(jobId ? site.path(`/processes/${encodeURIComponent(jobId)}/fs`) : '');
	let isNode = $derived(runtimeMode === 'node' || runtimeMode === 'custom');
	let preferExt = $derived(isNode ? ['.js', '.mjs', '.cjs'] : ([] as string[]));
	let logOptions = $derived(
		jobId ? [{ value: `job:${jobId}`, label: `Process · ${jobId}` }] : ([] as { value: string; label: string }[])
	);

	function buildExec(): string {
		const e = entry.trim();
		const p = extraParams.trim();
		const local = p ? `${e} ${p}` : e;
		if (!jobId) return local;
		if (local.startsWith('./') || local.startsWith('/')) return local;
		return `./processes/${jobId}/${local}`;
	}

	function inferRuntimeFromEntry(path: string) {
		if (/\.(js|mjs|cjs)$/i.test(path) && runtimeMode === 'binary') {
			runtimeMode = 'node';
			artifactKind = 'zip';
		}
	}

	async function createProcess() {
		const name = jobName.trim();
		if (!name) {
			toasts.error('Name is required');
			return false;
		}
		busy = true;
		try {
			const r = await request<{ process?: ProcessInfo }>(site.path('/processes'), {
				method: 'POST',
				json: { name, start: false }
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not create process');
				return false;
			}
			proc = r.data.process ?? { name };
			entry = isNode ? 'server.js' : './server';
			logUnit = `job:${proc.name}`;
			return true;
		} catch (e) {
			toasts.error(errorMessage(e));
			return false;
		} finally {
			busy = false;
		}
	}

	async function doUpload() {
		if (!jobId || !upload) {
			toasts.error('Choose a file to upload');
			return false;
		}
		busy = true;
		try {
			const fd = new FormData();
			fd.set('file', upload);
			const url =
				artifactKind === 'zip'
					? site.path(`/processes/${encodeURIComponent(jobId)}/deploy-zip`)
					: site.path(`/processes/${encodeURIComponent(jobId)}/deploy`);
			const r = await request(url, { method: 'POST', body: fd });
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
			toasts.success(artifactKind === 'zip' ? 'Zip extracted' : 'Binary uploaded');
			return true;
		} catch (e) {
			toasts.error(errorMessage(e));
			return false;
		} finally {
			busy = false;
		}
	}

	async function saveRuntime() {
		if (!jobId) return false;
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
			const r = await request<{ process?: ProcessInfo }>(
				site.path(`/processes/${encodeURIComponent(jobId)}`),
				{ method: 'PATCH', json: payload }
			);
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not save');
				return false;
			}
			proc = r.data.process ?? proc;
			return true;
		} catch (e) {
			toasts.error(errorMessage(e));
			return false;
		} finally {
			busy = false;
		}
	}

	async function startProcess() {
		if (!jobId) return;
		busy = true;
		try {
			if (!(await saveRuntime())) return;
			const r = await request(site.path(`/processes/${encodeURIComponent(jobId)}/start`), {
				method: 'POST'
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Start failed');
				return;
			}
			started = true;
			logUnit = `job:${jobId}`;
			toasts.success('Process started');
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			busy = false;
		}
	}

	async function next() {
		if (step === 0) {
			if (!(await createProcess())) return;
			step = 1;
			return;
		}
		if (step === 1) {
			if (upload && !uploaded) {
				if (!(await doUpload())) return;
			}
			step = 2;
			return;
		}
		if (step === 2) {
			if (!(await saveRuntime())) return;
			step = 3;
		}
	}

	function back() {
		if (step > 0) step -= 1;
	}

	async function cancelWizard() {
		if (jobId && !started) {
			try {
				await request(site.path(`/processes/${encodeURIComponent(jobId)}`), { method: 'DELETE' });
			} catch {
				/* best-effort */
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
			<h3>Name</h3>
			<p class="hint">Background workers and queues. Creates <code>processes/&lt;name&gt;/</code>.</p>
			<label class="wbr-field">
				<span>Process name</span>
				<input bind:value={jobName} placeholder="worker" disabled={busy} spellcheck="false" />
			</label>
		</div>
	{:else if step === 1}
		<div class="pane">
			<h3>Upload</h3>
			<p class="hint">
				Optional — zip extracts into <code>{proc?.dir || 'processes/…'}/</code>. Or skip and set an existing path.
			</p>
			<div class="seg" role="group" aria-label="Artifact type">
				<button
					type="button"
					class={['seg-btn', artifactKind === 'zip' && 'on']}
					disabled={busy}
					onclick={() => {
						artifactKind = 'zip';
						uploaded = false;
					}}>Zip</button
				>
				<button
					type="button"
					class={['seg-btn', artifactKind === 'binary' && 'on']}
					disabled={busy}
					onclick={() => {
						artifactKind = 'binary';
						uploaded = false;
					}}>Binary</button
				>
			</div>
			<FileDrop
				bind:file={upload}
				label={artifactKind === 'zip' ? 'Choose a zip' : 'Choose a binary'}
				accept={artifactKind === 'zip' ? '.zip,application/zip' : undefined}
				disabled={busy}
				onselect={() => (uploaded = false)}
			/>
			{#if uploaded}
				<p class="ok">Upload complete.</p>
			{/if}
		</div>
	{:else if step === 2}
		<div class="pane">
			<h3>How it runs</h3>
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
				<input
					bind:value={entry}
					placeholder={isNode ? 'server.js' : './server'}
					disabled={busy}
					spellcheck="false"
				/>
			</label>
			{#if fsListUrl}
				<button type="button" class="wbr-btn wbr-btn-ghost sm" disabled={busy} onclick={() => (showPicker = !showPicker)}>
					<Icon name="search" size={14} />
					{showPicker ? 'Hide files' : 'Browse job folder'}
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
				<input bind:value={extraParams} placeholder="--flag value" disabled={busy} spellcheck="false" />
			</label>
			<p class="exec-preview">
				ExecStart preview: <code>{isNode ? `node ${buildExec()}` : buildExec()}</code>
			</p>
		</div>
	{:else}
		<div class="pane">
			<h3>Start</h3>
			<p class="hint">Starts the systemd service in the background. Use Stop later from the list.</p>
			<button class="wbr-btn" type="button" disabled={busy || !jobId} onclick={() => void startProcess()}>
				<Icon name="rocket" size={15} />
				{busy ? 'Starting…' : started ? 'Restart process' : 'Start process'}
			</button>
			{#if jobId && logUnit}
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
			<button type="button" class="wbr-btn" disabled={busy || !jobId} onclick={() => jobId && ondone?.(jobId)}>
				<Icon name="check" size={15} /> Done
			</button>
		{/if}
	</div>
</div>

<style>
	.wizard {
		display: grid;
		gap: 1.25rem;
	}
	.steps {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.step {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		opacity: 0.45;
		font-size: 0.85rem;
	}
	.step.on,
	.step.done {
		opacity: 1;
	}
	.num {
		display: inline-grid;
		place-items: center;
		width: 1.4rem;
		height: 1.4rem;
		border-radius: 999px;
		border: 1px solid var(--wbr-border, #ccc);
		font-size: 0.75rem;
	}
	.step.on .num {
		background: var(--wbr-accent, #2563eb);
		border-color: transparent;
		color: #fff;
	}
	.pane {
		display: grid;
		gap: 0.85rem;
	}
	.pane h3 {
		margin: 0;
		font-size: 1.05rem;
	}
	.hint {
		margin: 0;
		opacity: 0.75;
		font-size: 0.9rem;
	}
	.ok {
		margin: 0;
		color: var(--wbr-success, #16a34a);
		font-size: 0.9rem;
	}
	.seg {
		display: inline-flex;
		gap: 0.25rem;
	}
	.seg-btn {
		border: 1px solid var(--wbr-border, #ccc);
		background: transparent;
		padding: 0.35rem 0.75rem;
		border-radius: 6px;
		cursor: pointer;
	}
	.seg-btn.on {
		background: var(--wbr-accent, #2563eb);
		border-color: transparent;
		color: #fff;
	}
	.exec-preview {
		margin: 0;
		font-size: 0.85rem;
		opacity: 0.8;
	}
	.logs {
		margin-top: 0.5rem;
	}
	.nav {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.spacer {
		flex: 1;
	}
</style>
