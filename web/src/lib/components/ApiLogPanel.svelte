<script lang="ts">
	import { onMount, tick } from 'svelte';
	import Icon from './Icon.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { poll } from '#lib/stores/metrics.svelte.ts';

	type UnitOpt = { value: string; label: string };

	type Props = {
		unit: string;
		unitOptions?: UnitOpt[];
		lines?: number;
		follow?: boolean;
		compact?: boolean;
	};

	let {
		unit = $bindable(),
		unitOptions = [],
		lines = 150,
		follow = $bindable(true),
		compact = false
	}: Props = $props();

	const site = getSite();

	let text = $state('');
	let unitName = $state('');
	let loading = $state(false);
	let error = $state('');
	let pre: HTMLPreElement | undefined = $state();

	let allLines = $derived(text ? text.replace(/\n$/, '').split('\n') : []);

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

	onMount(() => {
		void load();
		return poll(() => {
			if (follow && !loading) void load();
		}, 3_000);
	});
</script>

<div class={['panel', compact && 'compact']}>
	<div class="toolbar">
		{#if unitOptions.length > 1}
			<label class="wbr-field compact">
				<span class="visually-hidden">Logs</span>
				<select bind:value={unit} onchange={() => void load()}>
					{#each unitOptions as u (u.value)}
						<option value={u.value}>{u.label}</option>
					{/each}
				</select>
			</label>
		{:else if unitName}
			<code class="unit">{unitName}</code>
		{/if}
		<label class="toggle">
			<input type="checkbox" bind:checked={follow} onchange={() => follow && load()} />
			Follow
			{#if follow}<span class="live" aria-hidden="true"></span>{/if}
		</label>
		<button type="button" class="wbr-btn wbr-btn-ghost sm" disabled={loading} onclick={() => void load()}>
			<Icon name="refresh" size={14} />
			{loading ? '…' : 'Refresh'}
		</button>
	</div>
	{#if error}<p class="wbr-error pad" role="alert">{error}</p>{/if}
	<pre class="log" bind:this={pre} role="log" aria-label="API logs">{#if allLines.length === 0}<span class="empty"
				>{loading ? 'Loading…' : '(no log lines yet — start the API to see output)'}</span
			>{:else}{#each allLines as line, i (i)}<span class={['line', lineTone(line)]}>{line}</span>
{/each}{/if}</pre>
</div>

<style>
	.panel {
		border: 1px solid var(--border);
		border-radius: var(--radius);
		overflow: hidden;
		background: var(--surface);
	}

	.toolbar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem;
		padding: 0.55rem 0.75rem;
		border-bottom: 1px solid var(--border);
	}

	.compact .log {
		height: 14rem;
	}

	.unit {
		font-size: 0.75rem;
		color: var(--muted);
	}

	.toggle {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		font-size: 0.84rem;
		font-weight: 600;
		color: var(--muted);
		margin-left: auto;
	}

	.live {
		width: 0.5rem;
		height: 0.5rem;
		border-radius: 50%;
		background: var(--ok);
		animation: pulse 1.4s ease-in-out infinite;
	}

	.pad {
		margin: 0;
		padding: 0.5rem 0.75rem;
	}

	.log {
		margin: 0;
		padding: 0.75rem;
		height: 18rem;
		overflow: auto;
		background: #0d1117;
		color: #c9d1d9;
		font-family: var(--mono);
		font-size: 0.78rem;
		line-height: 1.55;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
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

	@keyframes pulse {
		50% {
			opacity: 0.35;
		}
	}
</style>
