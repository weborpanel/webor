<script lang="ts">
	import { goto } from '$app/navigation';
	import Modal from './Modal.svelte';
	import Icon from './Icon.svelte';
	import { api, errorMessage } from '#lib/api.ts';
	import { overview } from '#lib/stores/overview.svelte.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';

	type Props = { open: boolean };

	let { open = $bindable() }: Props = $props();

	let name = $state('');
	let kind = $state<'static' | 'app'>('static');
	let submitting = $state(false);
	let error = $state('');

	const HOST_RE = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$/;

	let domain = $derived(name.trim().toLowerCase());
	let invalid = $derived(domain.length > 0 && !HOST_RE.test(domain));

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		if (!HOST_RE.test(domain)) {
			error = 'Enter a valid hostname, e.g. example.com';
			return;
		}
		submitting = true;
		try {
			await api('/api/sites', { method: 'POST', json: { name: domain, type: kind } });
			toasts.success(`Created ${domain}`);
			open = false;
			name = '';
			void overview.refresh();
			await goto(`/sites/${encodeURIComponent(domain)}`);
		} catch (err) {
			error = errorMessage(err, 'Could not add site');
		} finally {
			submitting = false;
		}
	}
</script>

<Modal
	bind:open
	title="Add site"
	description="Creates the site user, directories and Caddy route."
	locked={submitting}
	onclose={() => (error = '')}
>
	<form id="add-site-form" class="form" onsubmit={submit}>
		<label class="wbr-field">
			<span>Domain</span>
			<!-- svelte-ignore a11y_autofocus -->
			<input
				bind:value={name}
				placeholder="example.com"
				autocomplete="off"
				spellcheck="false"
				required
				autofocus
				aria-invalid={invalid}
			/>
			{#if invalid}
				<span class="hint bad">Lowercase letters, digits and hyphens, with at least one dot.</span>
			{:else}
				<span class="hint">Point its DNS A record at this server, then enable HTTPS from Domains &amp; SSL.</span>
			{/if}
		</label>

		<fieldset>
			<legend>Type</legend>
			<div class="types">
				<label class={['type', kind === 'static' && 'on']}>
					<input type="radio" name="kind" value="static" bind:group={kind} />
					<Icon name="file" size={18} />
					<span>
						<strong>Static</strong>
						<small>HTML, assets, SPA builds served by Caddy</small>
					</span>
				</label>
				<label class={['type', kind === 'app' && 'on']}>
					<input type="radio" name="kind" value="app" bind:group={kind} />
					<Icon name="rocket" size={18} />
					<span>
						<strong>App</strong>
						<small>A binary or git build run as a systemd service</small>
					</span>
				</label>
			</div>
		</fieldset>

		{#if error}<p class="wbr-error" role="alert">{error}</p>{/if}
	</form>

	{#snippet footer()}
		<button type="button" class="wbr-btn wbr-btn-ghost" disabled={submitting} onclick={() => (open = false)}>
			Cancel
		</button>
		<button type="submit" form="add-site-form" class="wbr-btn" disabled={submitting || invalid}>
			{submitting ? 'Creating…' : 'Create site'}
		</button>
	{/snippet}
</Modal>

<style>
	.form {
		display: grid;
		gap: 1rem;
	}

	.hint.bad {
		color: var(--danger);
	}

	fieldset {
		border: 0;
		padding: 0;
		margin: 0;
	}

	legend {
		font-weight: 700;
		font-size: 0.88rem;
		margin-bottom: 0.35rem;
	}

	.types {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.6rem;
	}

	.type {
		position: relative;
		display: flex;
		gap: 0.6rem;
		align-items: flex-start;
		padding: 0.75rem;
		border: 1px solid var(--border-strong);
		border-radius: var(--radius);
		cursor: pointer;
		color: var(--muted);
	}

	.type:hover {
		border-color: var(--accent);
	}

	.type.on {
		border-color: var(--accent);
		background: var(--accent-soft);
		color: var(--accent);
		box-shadow: 0 0 0 1px var(--accent);
	}

	.type:has(input:focus-visible) {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}

	.type input {
		position: absolute;
		opacity: 0;
		pointer-events: none;
	}

	.type span {
		display: grid;
		gap: 0.1rem;
	}

	.type strong {
		color: var(--text);
		font-size: 0.92rem;
	}

	.type small {
		font-size: 0.78rem;
		color: var(--muted);
		line-height: 1.35;
	}

	@media (max-width: 480px) {
		.types {
			grid-template-columns: 1fr;
		}
	}
</style>
