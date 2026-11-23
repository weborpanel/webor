<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from './Icon.svelte';
	import { request, errorMessage } from '#lib/api.ts';

	type Entry = { name: string; type: string; size?: number };

	type Props = {
		listUrl: string;
		value?: string;
		preferExt?: string[];
		disabled?: boolean;
		onselect?: (path: string) => void;
		onclose?: () => void;
	};

	let { listUrl, value = '', preferExt = [], disabled = false, onselect, onclose }: Props = $props();

	let cwd = $state('');
	let entries = $state.raw<Entry[]>([]);
	let loading = $state(false);
	let error = $state('');

	function joinPath(dir: string, name: string): string {
		if (!dir) return name;
		return `${dir.replace(/\/$/, '')}/${name}`;
	}

	function parentPath(p: string): string {
		const parts = p.split('/').filter(Boolean);
		parts.pop();
		return parts.join('/');
	}

	function isPreferred(name: string): boolean {
		if (!preferExt.length) return false;
		const lower = name.toLowerCase();
		return preferExt.some((ext) => lower.endsWith(ext.toLowerCase()));
	}

	async function load(path = cwd) {
		loading = true;
		error = '';
		try {
			const q = path ? `?path=${encodeURIComponent(path)}` : '';
			const r = await request<{ entries?: Entry[] }>(`${listUrl}${q}`);
			if (!r.ok) {
				error = r.data.error || 'Could not list files';
				entries = [];
				return;
			}
			cwd = path;
			const list = r.data.entries ?? [];
			list.sort((a, b) => {
				if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
				const ap = isPreferred(a.name) ? 0 : 1;
				const bp = isPreferred(b.name) ? 0 : 1;
				if (ap !== bp) return ap - bp;
				return a.name.localeCompare(b.name);
			});
			entries = list;
		} catch (e) {
			error = errorMessage(e);
			entries = [];
		} finally {
			loading = false;
		}
	}

	function openDir(name: string) {
		void load(joinPath(cwd, name));
	}

	function pickFile(name: string) {
		const path = joinPath(cwd, name);
		onselect?.(path.startsWith('./') ? path : path.includes('/') || !path.startsWith('.') ? path : `./${path}`);
		onclose?.();
	}

	onMount(() => {
		if (listUrl && !disabled) void load('');
	});
</script>

<div class="picker" role="dialog" aria-label="Select entry file">
	<div class="bar">
		<button
			type="button"
			class="wbr-btn-ghost sm"
			disabled={disabled || loading || !cwd}
			onclick={() => void load(parentPath(cwd))}
			aria-label="Up"
		>
			<Icon name="chevronLeft" size={14} />
		</button>
		<code class="path">apis/…/{cwd || ''}</code>
		<button type="button" class="wbr-btn-ghost sm" disabled={disabled} onclick={() => onclose?.()} aria-label="Close">
			<Icon name="x" size={14} />
		</button>
	</div>
	{#if error}
		<p class="err">{error}</p>
	{:else if loading}
		<p class="muted">Loading…</p>
	{:else if !entries.length}
		<p class="muted">Empty directory. Deploy a zip or binary first.</p>
	{:else}
		<ul class="list">
			{#each entries as e (e.name)}
				<li class={['row', e.type === 'file' && isPreferred(e.name) && 'prefer', value === joinPath(cwd, e.name) && 'sel']}>
					{#if e.type === 'dir'}
						<button type="button" class="item" disabled={disabled} onclick={() => openDir(e.name)}>
							<Icon name="folder" size={14} />
							<span>{e.name}/</span>
						</button>
					{:else}
						<button type="button" class="item" disabled={disabled} onclick={() => pickFile(e.name)}>
							<Icon name="file" size={14} />
							<span>{e.name}</span>
						</button>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.picker {
		display: grid;
		gap: 0.5rem;
		padding: 0.65rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--surface-2, var(--panel));
		max-height: 16rem;
		overflow: auto;
	}

	.bar {
		display: flex;
		align-items: center;
		gap: 0.35rem;
	}

	.path {
		flex: 1;
		min-width: 0;
		font-size: 0.75rem;
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.15rem;
	}

	.item {
		display: flex;
		align-items: center;
		gap: 0.45rem;
		width: 100%;
		padding: 0.4rem 0.5rem;
		border: 0;
		border-radius: 6px;
		background: transparent;
		color: var(--text);
		font: inherit;
		font-size: 0.86rem;
		text-align: left;
		cursor: pointer;
	}

	.item:hover {
		background: var(--accent-soft);
	}

	.prefer .item {
		font-weight: 600;
	}

	.sel .item {
		background: var(--accent-soft);
		color: var(--accent);
	}

	.muted,
	.err {
		margin: 0;
		font-size: 0.84rem;
	}

	.muted {
		color: var(--muted);
	}

	.err {
		color: var(--danger, #c44);
	}
</style>
