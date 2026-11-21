<script lang="ts" generics="A">
	import { untrack } from 'svelte';
	import type { ApprovalFlow } from '#lib/approval.svelte.ts';
	import CopyField from './CopyField.svelte';
	import Icon from './Icon.svelte';

	type Props = {
		flow: ApprovalFlow<A>;
		/** Extra context shown after the operation name. */
		detail?: string;
	};

	let { flow, detail }: Props = $props();

	let id = $derived(flow.id.trim());

	$effect(() => {
		if (!flow.required || !id) return;
		untrack(() => void flow.poll());
		const timer = setInterval(() => void flow.poll(), 2000);
		return () => clearInterval(timer);
	});
</script>

<div class="approval" role="status">
	<span class="icon"><Icon name="shield" size={18} /></span>
	<div class="body">
		<p class="title">
			Root approval needed
			{#if flow.operation}for <code>{flow.operation}</code>{/if}
			{#if detail}<span class="detail">({detail})</span>{/if}
		</p>
		{#if id}
			<p class="wbr-muted">Run this on the server. The panel continues automatically once approved.</p>
			<CopyField value={`sudo wbr approve ${id}`} />
			<p class="state">
				{#if flow.status === 'approved' || flow.autoBusy}
					<Icon name="check" size={14} /> Approved, finishing…
				{:else}
					<span class="spinner" aria-hidden="true"></span> Waiting for approval…
				{/if}
			</p>
		{:else if flow.hint}
			<p class="wbr-muted hint">{flow.hint}</p>
		{/if}
		{#if !id && (flow.operation || flow.pending)}
			<div>
				<button class="wbr-btn sm" type="button" disabled={flow.busy} onclick={() => flow.requestApproval()}>
					{flow.busy ? 'Working…' : 'Create approval'}
				</button>
			</div>
		{/if}
	</div>
	<button type="button" class="dismiss" aria-label="Cancel approval" onclick={() => flow.clear()}>
		<Icon name="x" size={14} />
	</button>
</div>

<style>
	.approval {
		display: grid;
		grid-template-columns: auto 1fr auto;
		gap: 0.75rem;
		margin: 0 0 1rem;
		padding: 0.85rem 1rem;
		border: 1px solid color-mix(in srgb, var(--warn) 35%, var(--border));
		border-radius: var(--radius);
		background: var(--warn-soft);
	}

	.icon {
		color: var(--warn);
		padding-top: 0.1rem;
	}

	.body {
		display: grid;
		gap: 0.5rem;
		min-width: 0;
	}

	.body p {
		margin: 0;
		font-size: 0.88rem;
	}

	.title {
		font-weight: 800;
	}

	.detail {
		font-weight: 600;
		color: var(--muted);
	}

	.hint {
		white-space: pre-wrap;
	}

	.state {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		font-weight: 700;
		color: var(--muted);
	}

	.spinner {
		width: 0.8rem;
		height: 0.8rem;
		border-radius: 50%;
		border: 2px solid var(--border-strong);
		border-top-color: var(--warn);
		animation: spin 0.8s linear infinite;
	}

	.dismiss {
		align-self: start;
		background: none;
		border: 0;
		color: var(--subtle);
		cursor: pointer;
		padding: 0.2rem;
		border-radius: 4px;
	}

	.dismiss:hover {
		background: color-mix(in srgb, var(--warn) 15%, transparent);
		color: var(--text);
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
