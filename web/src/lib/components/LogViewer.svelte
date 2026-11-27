<script lang="ts">
	import { onMount, tick } from 'svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import { request, errorMessage } from '#lib/api.ts';
	import { formatBytes } from '#lib/format.ts';
	import { formatInTimeZone, remapLogText } from '#lib/logtime.ts';
	import { poll } from '#lib/stores/metrics.svelte.ts';

	type LogFile = {
		name: string;
		stream: string;
		kind: string;
		date?: string;
		index?: number;
		size: number;
		mtime: string;
	};

	type StreamOpt = { value: string; label: string };

	type Props = {
		listUrl: string;
		readUrl: (file: string, lines: number) => string;
		streams: StreamOpt[];
		downloadPrefix?: string;
		emptyTitle?: string;
		emptyDescription?: string;
	};

	let {
		listUrl,
		readUrl,
		streams,
		downloadPrefix = 'logs',
		emptyTitle = 'No streams to show',
		emptyDescription = 'Logs appear here once a stream is available.'
	}: Props = $props();

	const LINE_OPTIONS = [100, 200, 500];

	let stream = $state('');
	let file = $state('');
	let lines = $state(200);
	let text = $state('');
	let activeFile = $state.raw<LogFile | null>(null);
	let allFiles = $state.raw<LogFile[]>([]);
	let loading = $state(false);
	let listing = $state(false);
	let error = $state('');
	let follow = $state(false);
	let filter = $state('');
	let wrap = $state(true);
	let lastLoaded = $state<Date | null>(null);
	let timezone = $state('');
	let pre: HTMLPreElement | undefined;

	let streamFiles = $derived(allFiles.filter((f) => f.stream === stream));

	/** Raw file text kept for download; display remaps timestamps into panel TZ. */
	let displayText = $derived(remapLogText(text, timezone || undefined));
	let allLines = $derived(displayText ? displayText.replace(/\n$/, '').split('\n') : []);
	let shown = $derived.by(() => {
		const q = filter.trim().toLowerCase();
		return q ? allLines.filter((l) => l.toLowerCase().includes(q)) : allLines;
	});
	let updatedLabel = $derived(
		lastLoaded
			? timezone
				? formatInTimeZone(lastLoaded, timezone, {
						hour: '2-digit',
						minute: '2-digit',
						second: '2-digit',
						hourCycle: 'h23'
					})
				: lastLoaded.toLocaleTimeString()
			: ''
	);

	function pickDefaultStream(list: StreamOpt[]) {
		return list.find((s) => s.value !== 'access')?.value ?? list[0]?.value ?? '';
	}

	function isCurrent(f: LogFile) {
		return f.kind === 'current' || (f.name.endsWith('.log') && !f.date);
	}

	function pickPreferredFile(list: LogFile[]) {
		const current = list.find(isCurrent);
		if (current) return current.name;
		const sorted = [...list].sort((a, b) => {
			const d = (b.date || '').localeCompare(a.date || '');
			if (d) return d;
			return (b.index || 0) - (a.index || 0);
		});
		return sorted[0]?.name ?? '';
	}

	function fileLabel(f: LogFile) {
		const size = formatBytes(f.size);
		if (isCurrent(f)) return `Current (${size})`;
		const idx = f.index ? `.${f.index}` : '';
		return `${f.date || f.name}${idx} (${size})`;
	}

	function lineTone(l: string) {
		if (/\b(error|fatal|panic|fail(ed)?|exception)\b/i.test(l)) return 'err';
		if (/\b(warn(ing)?|deprecated)\b/i.test(l)) return 'warn';
		return '';
	}

	function ensureStream() {
		if (!streams.length) {
			stream = '';
			return;
		}
		if (!stream || !streams.some((s) => s.value === stream)) {
			stream = pickDefaultStream(streams);
		}
	}

	function ensureFile(list: LogFile[]) {
		if (list.some((f) => f.name === file)) return;
		file = pickPreferredFile(list);
	}

	async function loadFiles() {
		if (!listUrl) return;
		listing = true;
		try {
			const r = await request<{ files?: LogFile[]; timezone?: string }>(listUrl);
			if (!r.ok) {
				error = r.data.error || 'Could not list log files';
				allFiles = [];
				file = '';
				return;
			}
			error = '';
			if (r.data.timezone) timezone = r.data.timezone;
			allFiles = r.data.files ?? [];
			const filtered = allFiles.filter((f) => f.stream === stream);
			ensureFile(filtered);
		} catch (e) {
			error = errorMessage(e);
			allFiles = [];
			file = '';
		} finally {
			listing = false;
		}
	}

	async function load() {
		if (!file) {
			text = '';
			activeFile = null;
			return;
		}
		const atBottom = !pre || pre.scrollHeight - pre.scrollTop - pre.clientHeight < 40;
		loading = true;
		try {
			const r = await request<{ text?: string; file?: LogFile; timezone?: string }>(
				readUrl(file, lines)
			);
			if (!r.ok) {
				error = r.data.error || 'Could not load logs';
				text = '';
				activeFile = null;
				return;
			}
			error = '';
			text = r.data.text || '';
			if (r.data.timezone) timezone = r.data.timezone;
			activeFile = r.data.file ?? null;
			lastLoaded = new Date();
			if (atBottom) {
				await tick();
				if (pre) pre.scrollTop = pre.scrollHeight;
			}
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
		}
	}

	async function refresh() {
		await loadFiles();
		await load();
	}

	async function onStreamChange() {
		file = '';
		await loadFiles();
		await load();
	}

	function onFileChange() {
		void load();
	}

	function download() {
		const blob = new Blob([text], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		const stamp = new Date().toISOString().slice(0, 19);
		a.download = `${downloadPrefix}-${file || stream || 'log'}-${stamp}.log`.replaceAll('/', '-');
		a.click();
		URL.revokeObjectURL(url);
	}

	function bindPre(node: HTMLPreElement) {
		pre = node;
		return () => {
			if (pre === node) pre = undefined;
		};
	}

	onMount(() => {
		ensureStream();
		void request<{ timezone?: string }>('/api/panel').then((pr) => {
			if (pr.ok && pr.data.timezone) timezone = pr.data.timezone;
		});
		void (async () => {
			await loadFiles();
			await load();
		})();
		return poll(() => {
			if (follow && !loading) void load();
		}, 5_000);
	});
</script>

{#if streams.length === 0}
	<EmptyState icon="terminal" title={emptyTitle} description={emptyDescription} />
{:else}
	<Card flush>
		<div class="toolbar">
			<label class="wbr-field compact">
				<span class="visually-hidden">Stream</span>
				<select bind:value={stream} onchange={onStreamChange}>
					{#each streams as s (s.value)}
						<option value={s.value}>{s.label}</option>
					{/each}
				</select>
			</label>
			<label class="wbr-field compact">
				<span class="visually-hidden">File</span>
				<select bind:value={file} onchange={onFileChange} disabled={!streamFiles.length}>
					{#if streamFiles.length === 0}
						<option value="">No files</option>
					{:else}
						{#each streamFiles as f (f.name)}
							<option value={f.name}>{fileLabel(f)}</option>
						{/each}
					{/if}
				</select>
			</label>
			<label class="wbr-field compact">
				<span class="visually-hidden">Lines</span>
				<select bind:value={lines} onchange={load}>
					{#each LINE_OPTIONS as n (n)}
						<option value={n}>{n} lines</option>
					{/each}
				</select>
			</label>
			<label class="search">
				<Icon name="search" size={14} />
				<span class="visually-hidden">Filter lines</span>
				<input bind:value={filter} placeholder="Filter…" spellcheck="false" />
			</label>
			<div class="spacer"></div>
			<label class="toggle">
				<input type="checkbox" bind:checked={follow} onchange={() => follow && load()} />
				Follow
				{#if follow}<span class="live" aria-hidden="true"></span>{/if}
			</label>
			<label class="toggle">
				<input type="checkbox" bind:checked={wrap} />
				Wrap
			</label>
			{#if timezone}
				<span class="tz-badge" title="Panel schedule timezone (Account)">Times · {timezone}</span>
			{/if}
			<button type="button" class="wbr-btn wbr-btn-ghost sm" disabled={loading || listing} onclick={refresh}>
				<Icon name="refresh" size={14} />
				{loading || listing ? 'Loading…' : 'Refresh'}
			</button>
			<button type="button" class="wbr-btn wbr-btn-ghost sm" disabled={!text} onclick={download} aria-label="Download log">
				<Icon name="download" size={14} />
			</button>
		</div>

		{#if error}<p class="wbr-error pad" role="alert">{error}</p>{/if}

		<pre class={['log', wrap && 'wrap']} {@attach bindPre} role="log" aria-label="Log output">{#if shown.length === 0}<span
					class="empty">{loading || listing ? 'Loading…' : filter ? 'No lines match the filter.' : '(no log lines)'}</span
				>{:else}{#each shown as line, i (i)}<span class={['line', lineTone(line)]}>{line}</span>
{/each}{/if}</pre>

		{#snippet footer()}
			<span class="foot">
				{#if activeFile}<code>{activeFile.name}</code>{:else if file}<code>{file}</code>{/if}
				<span>
					{shown.length}{filter ? ` of ${allLines.length}` : ''} lines
					{#if updatedLabel}· updated {updatedLabel}{#if timezone} {timezone}{/if}{/if}
				</span>
			</span>
		{/snippet}
	</Card>
{/if}

<style>
	.toolbar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem;
		padding: 0.75rem 1rem;
		border-bottom: 1px solid var(--border);
	}

	.compact select {
		padding-block: 0.35rem;
		font-size: 0.85rem;
	}

	.search {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		padding: 0 0.6rem;
		border: 1px solid var(--border-strong);
		border-radius: var(--radius-sm);
		background: var(--surface);
		color: var(--subtle);
	}

	.search:focus-within {
		border-color: var(--accent);
	}

	.search input {
		border: 0;
		outline: 0;
		background: transparent;
		color: var(--text);
		padding: 0.38rem 0;
		font: inherit;
		font-size: 0.85rem;
		width: 10rem;
	}

	.spacer {
		flex: 1;
	}

	.toggle {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		font-size: 0.84rem;
		font-weight: 600;
		color: var(--muted);
	}

	.live {
		width: 0.5rem;
		height: 0.5rem;
		border-radius: 50%;
		background: var(--ok);
		animation: pulse 1.4s ease-in-out infinite;
	}

	.tz-badge {
		font-size: 0.75rem;
		font-weight: 600;
		color: var(--subtle);
		white-space: nowrap;
	}

	.pad {
		padding: 0.75rem 1rem;
		margin: 0;
	}

	.log {
		margin: 0;
		padding: 0.85rem 1rem;
		height: min(65vh, 40rem);
		overflow: auto;
		background: #0d1117;
		color: #c9d1d9;
		font-family: var(--mono);
		font-size: 0.78rem;
		line-height: 1.55;
		white-space: pre;
	}

	.log.wrap {
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}

	.log:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}

	.line.err {
		color: #ff7b72;
	}

	.line.warn {
		color: #e3b341;
	}

	.empty {
		color: #8b949e;
	}

	.foot {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
		font-size: 0.78rem;
		color: var(--subtle);
	}

	@keyframes pulse {
		50% {
			opacity: 0.35;
		}
	}
</style>
