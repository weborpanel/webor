<script lang="ts">
	import { onMount } from 'svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import FileDrop from '#lib/components/FileDrop.svelte';
	import Modal from '#lib/components/Modal.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage, sitePath } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';
	import { confirmDialog } from '#lib/stores/confirm.svelte.ts';
	import { formatBytes } from '#lib/format.ts';

	const site = getSite();

	type Entry = { name: string; type: 'file' | 'dir'; size: number; mtime: number; mode?: string };

	let cwd = $state('');
	let entries = $state.raw<Entry[]>([]);
	let loading = $state(true);
	let loadError = $state('');
	let selected = $state<string | null>(null);

	let uploadOpen = $state(false);
	let uploadFile = $state<File | null>(null);
	let uploading = $state(false);

	let mkdirOpen = $state(false);
	let mkdirName = $state('');
	let mkdirBusy = $state(false);

	let renameOpen = $state(false);
	let renameFrom = $state('');
	let renameTo = $state('');
	let renameBusy = $state(false);

	let extractOpen = $state(false);
	let extractFile = $state<File | null>(null);
	let extracting = $state(false);

	let editorOpen = $state(false);
	let editorPath = $state('');
	let editorName = $state('');
	let editorContent = $state('');
	let editorOriginal = $state('');
	let editorLoading = $state(false);
	let editorSaving = $state(false);
	let editorIsNew = $state(false);
	let newFileOpen = $state(false);
	let newFileName = $state('');

	const editorDirty = $derived(editorContent !== editorOriginal);
	const editorBytes = $derived(new TextEncoder().encode(editorContent).length);
	const maxEditBytes = 1024 * 1024;

	let crumbs = $derived.by(() => {
		const parts = cwd.split('/').filter(Boolean);
		const out: { label: string; path: string }[] = [{ label: site.root || 'public', path: '' }];
		let acc = '';
		for (const p of parts) {
			acc = acc ? `${acc}/${p}` : p;
			out.push({ label: p, path: acc });
		}
		return out;
	});

	function joinPath(dir: string, name: string) {
		return dir ? `${dir}/${name}` : name;
	}

	function isArchive(name: string) {
		const n = name.toLowerCase();
		return n.endsWith('.zip') || n.endsWith('.tar') || n.endsWith('.tar.gz') || n.endsWith('.tgz');
	}

	async function load(path = cwd) {
		loading = true;
		loadError = '';
		selected = null;
		try {
			const q = new URLSearchParams();
			if (path) q.set('path', path);
			const r = await request<{ entries?: Entry[]; path?: string }>(
				`${sitePath(site.name, '/fs')}?${q}`
			);
			if (!r.ok) {
				loadError = r.data.error || 'Could not list files';
				entries = [];
				return;
			}
			cwd = path;
			entries = [...(r.data.entries || [])].sort((a, b) => {
				if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
				return a.name.localeCompare(b.name);
			});
		} catch (e) {
			loadError = errorMessage(e);
			entries = [];
		} finally {
			loading = false;
		}
	}

	function openDir(name: string) {
		void load(joinPath(cwd, name));
	}

	function goCrumb(path: string) {
		void load(path);
	}

	async function doUpload() {
		if (!uploadFile) {
			toasts.error('Choose a file');
			return;
		}
		uploading = true;
		try {
			const fd = new FormData();
			fd.set('path', cwd);
			fd.set('file', uploadFile);
			const r = await request<{ path?: string }>(sitePath(site.name, '/fs/upload'), {
				method: 'POST',
				body: fd
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Upload failed');
				return;
			}
			toasts.success(`Uploaded ${r.data.path || uploadFile.name}`);
			uploadOpen = false;
			uploadFile = null;
			await load();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			uploading = false;
		}
	}

	async function doMkdir() {
		const name = mkdirName.trim().replace(/^\/+|\/+$/g, '');
		if (!name || name.includes('..') || name.includes('/')) {
			toasts.error('Enter a single folder name');
			return;
		}
		mkdirBusy = true;
		try {
			const path = joinPath(cwd, name);
			const r = await request(sitePath(site.name, '/fs/mkdir'), {
				method: 'POST',
				json: { path }
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not create folder');
				return;
			}
			toasts.success(`Created ${path}`);
			mkdirOpen = false;
			mkdirName = '';
			await load();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			mkdirBusy = false;
		}
	}

	async function doRename() {
		const toName = renameTo.trim();
		if (!toName || toName.includes('..')) {
			toasts.error('Enter a valid name');
			return;
		}
		renameBusy = true;
		try {
			const from = joinPath(cwd, renameFrom);
			const to = joinPath(cwd, toName);
			const r = await request(sitePath(site.name, '/fs/rename'), {
				method: 'POST',
				json: { from, to }
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Rename failed');
				return;
			}
			toasts.success('Renamed');
			renameOpen = false;
			await load();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			renameBusy = false;
		}
	}

	async function doDelete(entry: Entry) {
		const path = joinPath(cwd, entry.name);
		const recursive = entry.type === 'dir';
		const ok = await confirmDialog({
			title: recursive ? `Delete folder ${entry.name}?` : `Delete ${entry.name}?`,
			message: recursive
				? 'All files inside this folder will be removed permanently.'
				: 'This file will be removed permanently.',
			confirmLabel: 'Delete',
			danger: true,
			typeToConfirm: recursive ? entry.name : undefined
		});
		if (!ok) return;
		try {
			const q = new URLSearchParams({ path });
			if (recursive) q.set('recursive', '1');
			const r = await request(`${sitePath(site.name, '/fs')}?${q}`, { method: 'DELETE' });
			if (!r.ok) {
				toasts.error(r.data.error || 'Delete failed');
				return;
			}
			toasts.success('Deleted');
			await load();
		} catch (e) {
			toasts.error(errorMessage(e));
		}
	}

	async function doExtract() {
		if (!extractFile) {
			toasts.error('Choose an archive');
			return;
		}
		if (!isArchive(extractFile.name)) {
			toasts.error('Use a .zip, .tar, .tar.gz or .tgz file');
			return;
		}
		extracting = true;
		try {
			const fd = new FormData();
			fd.set('dest', cwd);
			fd.set('file', extractFile);
			const r = await request(sitePath(site.name, '/fs/extract'), {
				method: 'POST',
				body: fd
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Extract failed');
				return;
			}
			toasts.success(`Extracted into ${site.root}/${cwd || ''}`);
			extractOpen = false;
			extractFile = null;
			await load();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			extracting = false;
		}
	}

	function download(entry: Entry) {
		const path = joinPath(cwd, entry.name);
		const q = new URLSearchParams({ path });
		window.open(`${sitePath(site.name, '/fs/raw')}?${q}`, '_blank');
	}

	function startRename(entry: Entry) {
		renameFrom = entry.name;
		renameTo = entry.name;
		renameOpen = true;
	}

	function isProbablyText(name: string) {
		const n = name.toLowerCase();
		if (n.endsWith('.min.js') || n.endsWith('.min.css') || n.endsWith('.map')) return false;
		return /\.(html?|css|js|mjs|cjs|json|md|txt|svg|xml|ya?ml|toml|env|conf|cfg|ini|sh|bash|py|go|rs|ts|tsx|jsx|svelte|vue|php|rb|sql|log|csv|tsv|htaccess|gitignore|dockerignore|editorconfig|nvmrc|npmrc)$/i.test(
			n
		);
	}

	async function openEditor(entry: Entry) {
		if (entry.type !== 'file') return;
		if (entry.size > maxEditBytes) {
			toasts.error('File is too large to edit in the panel — download it instead');
			return;
		}
		const path = joinPath(cwd, entry.name);
		editorOpen = true;
		editorIsNew = false;
		editorPath = path;
		editorName = entry.name;
		editorContent = '';
		editorOriginal = '';
		editorLoading = true;
		try {
			const q = new URLSearchParams({ path });
			const r = await request<{ content?: string; name?: string }>(
				`${sitePath(site.name, '/fs/content')}?${q}`
			);
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not open file');
				editorOpen = false;
				return;
			}
			editorContent = r.data.content ?? '';
			editorOriginal = editorContent;
			if (r.data.name) editorName = r.data.name;
		} catch (e) {
			toasts.error(errorMessage(e));
			editorOpen = false;
		} finally {
			editorLoading = false;
		}
	}

	function startNewFile() {
		newFileName = '';
		newFileOpen = true;
	}

	function createNewFile() {
		const name = newFileName.trim().replace(/^\/+|\/+$/g, '');
		if (!name || name.includes('..') || name.includes('/')) {
			toasts.error('Enter a single file name');
			return;
		}
		newFileOpen = false;
		editorIsNew = true;
		editorPath = joinPath(cwd, name);
		editorName = name;
		editorContent = '';
		editorOriginal = '';
		editorLoading = false;
		editorOpen = true;
	}

	async function saveEditor() {
		if (!editorPath) return;
		if (editorBytes > maxEditBytes) {
			toasts.error(`Content exceeds ${formatBytes(maxEditBytes)} editor limit`);
			return;
		}
		editorSaving = true;
		try {
			const r = await request(sitePath(site.name, '/fs/content'), {
				method: 'PUT',
				json: { path: editorPath, content: editorContent }
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Save failed');
				return;
			}
			editorOriginal = editorContent;
			editorIsNew = false;
			toasts.success(`Saved ${editorName}`);
			await load();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			editorSaving = false;
		}
	}

	function requestCloseEditor() {
		if (editorSaving) return;
		editorOpen = false;
	}

	async function onEditorModalClose() {
		if (editorSaving) {
			editorOpen = true;
			return;
		}
		if (!editorDirty) return;
		const ok = await confirmDialog({
			title: 'Discard changes?',
			message: `${editorName} has unsaved changes.`,
			confirmLabel: 'Discard',
			danger: true
		});
		if (!ok) {
			editorOpen = true;
			return;
		}
		editorContent = editorOriginal;
	}

	function onEditorKeydown(e: KeyboardEvent) {
		if (!editorOpen || editorLoading) return;
		if ((e.metaKey || e.ctrlKey) && e.key === 's') {
			e.preventDefault();
			void saveEditor();
		}
	}

	onMount(() => {
		void load('');
	});
</script>

<svelte:window onkeydown={onEditorKeydown} />

{#if site.isApp}
	<div class="note" role="note">
		<Icon name="rocket" size={16} />
		<p>
			App releases go through
			<a href={site.href('/deploy')}>Deploy</a>. This browser manages the site document root
			(<code>{site.root}/</code>) only — not <code>data/</code> or binaries.
		</p>
	</div>
{/if}

<Card title="Files" description={`Browsing ${site.root}/${cwd}`}>
	{#snippet actions()}
		<div class="toolbar">
			<button type="button" class="wbr-btn wbr-btn-ghost sm" disabled={loading} onclick={() => load()} aria-label="Refresh">
				<Icon name="refresh" size={14} />
			</button>
			<button type="button" class="wbr-btn wbr-btn-ghost sm" onclick={startNewFile}>
				<Icon name="file" size={14} /> New file
			</button>
			<button type="button" class="wbr-btn wbr-btn-ghost sm" onclick={() => (mkdirOpen = true)}>
				<Icon name="plus" size={14} /> Folder
			</button>
			<button type="button" class="wbr-btn wbr-btn-ghost sm" onclick={() => (uploadOpen = true)}>
				<Icon name="upload" size={14} /> Upload
			</button>
			<button type="button" class="wbr-btn sm" onclick={() => (extractOpen = true)}>
				<Icon name="box" size={14} /> Extract
			</button>
		</div>
	{/snippet}

	<nav class="crumbs" aria-label="Path">
		{#each crumbs as c, i (c.path)}
			{#if i > 0}<span class="sep">/</span>{/if}
			<button type="button" class={['crumb', c.path === cwd && 'current']} onclick={() => goCrumb(c.path)}>
				{c.label}
			</button>
		{/each}
	</nav>

	{#if loadError}
		<p class="wbr-error" role="alert">{loadError}</p>
	{/if}

	{#if loading && entries.length === 0}
		<Skeleton lines={5} />
	{:else if entries.length === 0}
		<EmptyState
			icon="file"
			title="Empty folder"
			description="Upload files or extract a zip/tar archive into this folder."
		>
			<button type="button" class="wbr-btn" onclick={() => (uploadOpen = true)}>
				<Icon name="upload" size={15} /> Upload
			</button>
		</EmptyState>
	{:else}
		<table class="listing">
			<thead>
				<tr>
					<th>Name</th>
					<th class="num">Size</th>
					<th class="date">Modified</th>
					<th class="acts"></th>
				</tr>
			</thead>
			<tbody>
				{#each entries as e (e.name)}
					<tr class={['row', selected === e.name && 'sel']}>
						<td>
							<button
								type="button"
								class="name"
								onclick={() => (e.type === 'dir' ? openDir(e.name) : (selected = e.name))}
								ondblclick={() =>
									e.type === 'dir'
										? openDir(e.name)
										: isProbablyText(e.name)
											? openEditor(e)
											: download(e)
								}
							>
								<Icon name={e.type === 'dir' ? 'folder' : 'file'} size={15} />
								<span>{e.name}</span>
							</button>
						</td>
						<td class="num">{e.type === 'dir' ? '—' : formatBytes(e.size)}</td>
						<td class="date">{e.mtime ? new Date(e.mtime * 1000).toLocaleString() : '—'}</td>
						<td class="acts">
							{#if e.type === 'file'}
								<button type="button" class="icon-btn" title="Edit" onclick={() => openEditor(e)}>
									<Icon name="pencil" size={14} />
								</button>
								<button type="button" class="icon-btn" title="Download" onclick={() => download(e)}>
									<Icon name="download" size={14} />
								</button>
							{/if}
							<button type="button" class="icon-btn" title="Rename" onclick={() => startRename(e)}>
								<Icon name="file" size={14} />
							</button>
							<button type="button" class="icon-btn danger" title="Delete" onclick={() => doDelete(e)}>
								<Icon name="trash" size={14} />
							</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</Card>

<Modal bind:open={uploadOpen} title="Upload file" locked={uploading}>
	<p class="wbr-muted dest">Destination: <code>{site.root}/{cwd ? `${cwd}/` : ''}</code></p>
	<FileDrop bind:file={uploadFile} label="Choose a file" disabled={uploading} />
	{#snippet footer()}
		<button type="button" class="wbr-btn wbr-btn-ghost" disabled={uploading} onclick={() => (uploadOpen = false)}>
			Cancel
		</button>
		<button type="button" class="wbr-btn" disabled={uploading || !uploadFile} onclick={doUpload}>
			{uploading ? 'Uploading…' : 'Upload'}
		</button>
	{/snippet}
</Modal>

<Modal bind:open={mkdirOpen} title="New folder" locked={mkdirBusy}>
	<label class="wbr-field">
		<span>Folder name</span>
		<input bind:value={mkdirName} placeholder="assets" required autocomplete="off" />
	</label>
	{#snippet footer()}
		<button type="button" class="wbr-btn wbr-btn-ghost" disabled={mkdirBusy} onclick={() => (mkdirOpen = false)}>
			Cancel
		</button>
		<button type="button" class="wbr-btn" disabled={mkdirBusy || !mkdirName.trim()} onclick={doMkdir}>
			{mkdirBusy ? 'Creating…' : 'Create'}
		</button>
	{/snippet}
</Modal>

<Modal bind:open={renameOpen} title="Rename" locked={renameBusy}>
	<label class="wbr-field">
		<span>New name</span>
		<input bind:value={renameTo} required autocomplete="off" />
	</label>
	{#snippet footer()}
		<button type="button" class="wbr-btn wbr-btn-ghost" disabled={renameBusy} onclick={() => (renameOpen = false)}>
			Cancel
		</button>
		<button type="button" class="wbr-btn" disabled={renameBusy || !renameTo.trim()} onclick={doRename}>
			{renameBusy ? 'Saving…' : 'Rename'}
		</button>
	{/snippet}
</Modal>

<Modal bind:open={extractOpen} title="Extract archive" locked={extracting}>
	<p class="wbr-muted dest">
		Extracts into <code>{site.root}/{cwd || ''}</code>. Supports zip, tar, tar.gz.
	</p>
	<FileDrop
		bind:file={extractFile}
		label="Choose archive"
		accept=".zip,.tar,.tar.gz,.tgz,application/zip,application/x-tar,application/gzip"
		disabled={extracting}
	/>
	{#snippet footer()}
		<button type="button" class="wbr-btn wbr-btn-ghost" disabled={extracting} onclick={() => (extractOpen = false)}>
			Cancel
		</button>
		<button type="button" class="wbr-btn" disabled={extracting || !extractFile} onclick={doExtract}>
			{extracting ? 'Extracting…' : 'Extract'}
		</button>
	{/snippet}
</Modal>

<Modal bind:open={newFileOpen} title="New file">
	<label class="wbr-field">
		<span>File name</span>
		<input
			bind:value={newFileName}
			placeholder="index.html"
			required
			autocomplete="off"
			onkeydown={(e) => {
				if (e.key === 'Enter') {
					e.preventDefault();
					createNewFile();
				}
			}}
		/>
	</label>
	{#snippet footer()}
		<button type="button" class="wbr-btn wbr-btn-ghost" onclick={() => (newFileOpen = false)}>Cancel</button>
		<button type="button" class="wbr-btn" disabled={!newFileName.trim()} onclick={createNewFile}>Create</button>
	{/snippet}
</Modal>

<Modal
	bind:open={editorOpen}
	title={editorIsNew ? `New file · ${editorName}` : editorName}
	description={`${site.root}/${editorPath}${editorDirty ? ' · unsaved' : ''}`}
	width="48rem"
	locked={editorSaving}
	onclose={() => {
		void onEditorModalClose();
	}}
>
	{#if editorLoading}
		<Skeleton lines={8} />
	{:else}
		<label class="wbr-field editor-field">
			<span class="visually-hidden">File contents</span>
			<textarea
				class="editor"
				bind:value={editorContent}
				spellcheck="false"
				wrap="off"
				rows={18}
				disabled={editorSaving}
				aria-label={`Edit ${editorName}`}
			></textarea>
		</label>
		<p class="editor-meta wbr-muted">
			{formatBytes(editorBytes)}
			{#if editorDirty}<span class="dirty">· modified</span>{/if}
			<span class="hint">Ctrl/⌘+S to save</span>
		</p>
	{/if}
	{#snippet footer()}
		<button type="button" class="wbr-btn wbr-btn-ghost" disabled={editorSaving} onclick={requestCloseEditor}>
			Close
		</button>
		<button
			type="button"
			class="wbr-btn"
			disabled={editorSaving || editorLoading || (!editorDirty && !editorIsNew)}
			onclick={saveEditor}
		>
			{editorSaving ? 'Saving…' : 'Save'}
		</button>
	{/snippet}
</Modal>

<style>
	.note {
		display: flex;
		gap: 0.65rem;
		align-items: flex-start;
		padding: 0.8rem 1rem;
		margin-bottom: 1rem;
		border-radius: var(--radius);
		background: var(--info-soft);
		color: var(--info);
	}

	.note p {
		margin: 0;
		color: var(--text);
		font-size: 0.9rem;
	}

	.toolbar {
		display: flex;
		flex-wrap: wrap;
		gap: 0.35rem;
	}

	.crumbs {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.15rem;
		margin-bottom: 0.85rem;
		font-size: 0.88rem;
	}

	.crumb {
		background: none;
		border: 0;
		padding: 0.15rem 0.35rem;
		border-radius: var(--radius-sm);
		font: inherit;
		font-weight: 700;
		color: var(--accent);
		cursor: pointer;
	}

	.crumb:hover {
		background: var(--accent-soft);
	}

	.crumb.current {
		color: var(--text);
		cursor: default;
	}

	.sep {
		color: var(--subtle);
	}

	.listing {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.9rem;
	}

	.listing th,
	.listing td {
		text-align: left;
		padding: 0.45rem 0.35rem;
		border-bottom: 1px solid var(--border);
	}

	.listing th {
		font-size: 0.75rem;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--muted);
	}

	.listing tr.sel {
		background: var(--accent-soft);
	}

	.name {
		display: inline-flex;
		align-items: center;
		gap: 0.45rem;
		background: none;
		border: 0;
		padding: 0;
		font: inherit;
		font-weight: 700;
		color: var(--text);
		cursor: pointer;
		text-align: left;
	}

	.num,
	.date {
		white-space: nowrap;
		color: var(--muted);
		font-variant-numeric: tabular-nums;
	}

	.acts {
		text-align: right;
		white-space: nowrap;
	}

	.icon-btn {
		background: none;
		border: 0;
		padding: 0.3rem;
		border-radius: var(--radius-sm);
		color: var(--muted);
		cursor: pointer;
	}

	.icon-btn:hover {
		background: var(--surface-3);
		color: var(--text);
	}

	.icon-btn.danger:hover {
		background: var(--danger-soft);
		color: var(--danger);
	}

	.dest {
		margin: 0 0 0.75rem;
		font-size: 0.88rem;
	}

	.editor-field {
		margin: 0;
	}

	.editor {
		width: 100%;
		min-height: min(60vh, 28rem);
		resize: vertical;
		font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
		font-size: 0.85rem;
		line-height: 1.45;
		tab-size: 2;
		white-space: pre;
		overflow: auto;
	}

	.editor-meta {
		display: flex;
		flex-wrap: wrap;
		gap: 0.35rem 0.75rem;
		margin: 0.55rem 0 0;
		font-size: 0.8rem;
	}

	.dirty {
		color: var(--warn, #b45309);
		font-weight: 700;
	}

	.hint {
		margin-left: auto;
		opacity: 0.75;
	}

</style>
