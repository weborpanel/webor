<script lang="ts">
	import Icon from './Icon.svelte';
	import { eventFailed, eventIcon, eventSite, humanizeOperation } from '#lib/activity.ts';
	import { relativeTime } from '#lib/format.ts';
	import type { AuditEvent } from '#lib/types.ts';

	type Props = {
		events: AuditEvent[];
		/** Show raw args under each entry. */
		detailed?: boolean;
	};

	let { events, detailed = false }: Props = $props();

	function argsText(e: AuditEvent): string {
		const entries = Object.entries(e.args ?? {}).filter(([, v]) => v !== '' && v != null);
		return entries.map(([k, v]) => `${k}=${typeof v === 'object' ? JSON.stringify(v) : String(v)}`).join('  ');
	}
</script>

<ol class="feed">
	{#each events as e (e.id)}
		{@const site = eventSite(e)}
		{@const failed = eventFailed(e)}
		<li>
			<span class={['icon', failed && 'failed']}><Icon name={eventIcon(e.operation)} size={14} /></span>
			<div class="body">
				<div class="line">
					<strong>{humanizeOperation(e.operation)}</strong>
					{#if site}
						<a href={`/sites/${encodeURIComponent(site)}`}>{site}</a>
					{/if}
					{#if failed}<span class="wbr-badge danger">failed</span>{/if}
				</div>
				<div class="meta">
					<time datetime={e.ts} title={new Date(e.ts).toLocaleString()}>{relativeTime(e.ts)}</time>
					<span>· {e.actor}</span>
					{#if detailed && e.result && e.result !== 'ok'}<span>· {e.result}</span>{/if}
				</div>
				{#if detailed}
					{@const args = argsText(e)}
					{#if args}<code class="args">{args}</code>{/if}
				{/if}
			</div>
		</li>
	{/each}
</ol>

<style>
	.feed {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
	}

	li {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 0.7rem;
		padding: 0.6rem 0;
		position: relative;
	}

	li + li {
		border-top: 1px solid var(--border);
	}

	.icon {
		display: grid;
		place-items: center;
		width: 1.75rem;
		height: 1.75rem;
		border-radius: 8px;
		background: var(--surface-3);
		color: var(--muted);
	}

	.icon.failed {
		background: var(--danger-soft);
		color: var(--danger);
	}

	.body {
		min-width: 0;
		display: grid;
		gap: 0.1rem;
	}

	.line {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.88rem;
	}

	.line a {
		font-weight: 600;
		text-decoration: none;
		overflow-wrap: anywhere;
	}

	.meta {
		display: flex;
		flex-wrap: wrap;
		gap: 0.3rem;
		font-size: 0.78rem;
		color: var(--subtle);
	}

	.args {
		margin-top: 0.25rem;
		display: block;
		font-size: 0.75rem;
		color: var(--muted);
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
</style>
