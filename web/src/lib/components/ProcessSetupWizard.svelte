<script lang="ts">
	import Icon from './Icon.svelte';
	import FileDrop from './FileDrop.svelte';
	import EntryPicker from './EntryPicker.svelte';
	import ApiLogPanel from './ApiLogPanel.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';

	type ArtifactKind = 'zip' | 'binary';
	type ProcessInfo = {
		name?: string;
		exec?: string;
		dir?: string;
		unit?: string;
		active?: boolean;
	};

	type Props = {
		ondone?: (jobName: string) => void;
		oncancel?: () => void;
	};

	let { ondone, oncancel }: Props = $props();

	const site = getSite();
	const steps = ['Name', 'Upload', 'Entry', 'Start'] as const;

	let step = $state(0);
	let busy = $state(false);
	let proc = $state.raw<ProcessInfo | null>(null);

	let jobName = $state('');
	let artifactKind = $state<ArtifactKind>('zip');
	let upload = $state<File | null>(null);
	let uploaded = $state(false);

	let entry = $state('./server');
	let showPicker = $state(true);
	let started = $state(false);
	let logUnit = $state('');

	let jobId = $derived(proc?.name || '');
	let fsListUrl = $derived(jobId ? site.path(`/processes/${encodeURIComponent(jobId)}/fs`) : '');
	let execPreview = $derived.by(() => {
		const e = entry.trim();
		if (!e) return '';
		if (e.startsWith('./') || e.startsWith('/')) return e;
		return jobId ? `./processes/${jobId}/${e}` : e;
	});
	let logOptions = $derived(
		jobId ? [{ value: `job:${jobId}`, label: `Process · ${jobId}` }] : ([] as { value: string; label: string }[])
	);

	function normalizeEntry(path: string): string {
		const p = path.trim().replace(/^\.\//, '');
		if (!jobId) return path.trim();
		if (path.startsWith('./processes/') || path.startsWith('/')) return path.trim();
		return `./processes/${jobId}/${p}`;
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
			entry = `./processes/${proc.name}/server`;
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
			toasts.success(artifactKind === 'zip' ? 'Zip extracted' : 'Binary uploaded');
			return true;
		} catch (e) {
			toasts.error(errorMessage(e));
			return false;
		} finally {
			busy = false;
		}
	}

	async function saveEntry() {
		if (!jobId) return false;
		const exec = normalizeEntry(entry);
		busy = true;
		try {
			const r = await request<{ process?: ProcessInfo }>(
				site.path(`/processes/${encodeURIComponent(jobId)}`),
				{ method: 'PATCH', json: { exec } }
			);
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not save');
				return false;
			}
			proc = r.data.process ?? proc;
			entry = exec;
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
			if (!(await saveEntry())) return;
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
			<h3>Entry</h3>
			<label class="wbr-field">
				<span>Exec path</span>
				<input bind:value={entry} placeholder="./processes/worker/server" disabled={busy} spellcheck="false" />
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
							disabled={busy}
							onselect={(path) => {
								entry = normalizeEntry(path);
							}}
						/>
					{/key}
				{/if}
			{/if}
			<p class="exec-preview">ExecStart: <code>{execPreview || '—'}</code></p>
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
