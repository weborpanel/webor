<script lang="ts">
	import { onMount, tick } from 'svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { poll } from '#lib/stores/metrics.svelte.ts';

	const site = getSite();

	type Unit = { value: string; label: string };

	let units = $derived.by((): Unit[] => {
		const list: Unit[] = [];
		if (site.isApp) {
			list.push({ value: 'app', label: site.socketProxy ? 'Proxyd' : 'App' });
			if (site.socketProxy) list.push({ value: 'backend', label: 'Backend' });
		}
		for (const c of site.cron) list.push({ value: `cron:${c}`, label: `Cron · ${c}` });
		for (const j of site.jobs) list.push({ value: `job:${j}`, label: `Job · ${j}` });
		return list;
	});

	function defaultUnit() {
		if (site.socketProxy) return 'backend';
		if (site.isApp) return 'app';
		if (site.cron.length) return `cron:${site.cron[0]}`;
		if (site.jobs.length) return `job:${site.jobs[0]}`;
		return '';
	}

	const LINE_OPTIONS = [100, 200, 500, 1000];

	let unit = $state(defaultUnit());
	let lines = $state(200);
	let text = $state('');
	let unitName = $state('');
	let loading = $state(false);
	let error = $state('');
	let follow = $state(false);
	let filter = $state('');
	let wrap = $state(true);
	let lastLoaded = $state<Date | null>(null);
	let pre: HTMLPreElement | undefined = $state();

	let allLines = $derived(text ? text.replace(/\n$/, '').split('\n') : []);
	let shown = $derived.by(() => {
		const q = filter.trim().toLowerCase();
		return q ? allLines.filter((l) => l.toLowerCase().includes(q)) : allLines;
	});

	function lineTone(l: string) {
		if (/\b(error|fatal|panic|fail(ed)?|exception)\b/i.test(l)) return 'err';
		if (/\b(warn(ing)?|deprecated)\b/i.test(l)) return 'warn';
		return '';
	}

	async function load() {
		if (!unit) return;
		const atBottom = !pre || pre.scrollHeight - pre.scrollTop - pre.clientHeight < 40;
		loading = true;
		try {
			const q = new URLSearchParams({ unit, lines: String(lines) });
			const r = await request<{ text?: string; unit?: string }>(site.path(`/logs?${q}`));
			if (!r.ok) {
				error = r.data.error || 'Could not load logs';
				text = '';
				return;
			}
			error = '';
			text = r.data.text || '';
			unitName = r.data.unit || '';
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

	function download() {
		const blob = new Blob([text], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `${site.name}-${unit.replace(':', '-')}-${new Date().toISOString().slice(0, 19)}.log`;
		a.click();
		URL.revokeObjectURL(url);
	}

	onMount(() => {
		void load();
		return poll(() => {
			if (follow && !loading) void load();
		}, 5_000);
	});
</script>

{#if units.length === 0}
	<EmptyState
		icon="terminal"
		title="No units to show"
		description="Static sites are served by Caddy. Logs appear here once the site has cron entries or jobs."
	/>
{:else}
	<Card flush>
		<div class="toolbar">
			<label class="wbr-field compact">
				<span class="visually-hidden">Unit</span>
				<select bind:value={unit} onchange={load}>
					{#each units as u (u.value)}
						<option value={u.value}>{u.label}</option>
					{/each}
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
			<button type="button" class="wbr-btn wbr-btn-ghost sm" disabled={loading} onclick={load}>
				<Icon name="refresh" size={14} />
				{loading ? 'Loading…' : 'Refresh'}
			</button>
			<button type="button" class="wbr-btn wbr-btn-ghost sm" disabled={!text} onclick={download} aria-label="Download log">
				<Icon name="download" size={14} />
			</button>
		</div>

		{#if error}<p class="wbr-error pad" role="alert">{error}</p>{/if}

		<pre class={['log', wrap && 'wrap']} bind:this={pre} role="log" aria-label="Log output">{#if shown.length === 0}<span
					class="empty">{loading ? 'Loading…' : filter ? 'No lines match the filter.' : '(no log lines)'}</span
				>{:else}{#each shown as line, i (i)}<span class={['line', lineTone(line)]}>{line}</span>
{/each}{/if}</pre>

		{#snippet footer()}
			<span class="foot">
				{#if unitName}<code>{unitName}</code>{/if}
				<span>
					{shown.length}{filter ? ` of ${allLines.length}` : ''} lines
					{#if lastLoaded}· updated {lastLoaded.toLocaleTimeString()}{/if}
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
