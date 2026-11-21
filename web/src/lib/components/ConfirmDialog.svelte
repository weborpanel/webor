<script lang="ts">
	import { confirmer } from '#lib/stores/confirm.svelte.ts';
	import Modal from './Modal.svelte';

	let typed = $state('');
	let current = $derived(confirmer.current);
	let canConfirm = $derived(!current?.typeToConfirm || typed.trim() === current.typeToConfirm);

	let open = $derived(!!current);

	function settle(ok: boolean) {
		if (ok && !canConfirm) return;
		typed = '';
		confirmer.settle(ok);
	}
</script>

<Modal
	bind:open={() => open, (v) => {
		if (!v && current) settle(false);
	}}
	title={current?.title ?? ''}
	width="27rem"
>
	{#if current}
		<form
			id="confirm-form"
			class="body"
			onsubmit={(e) => {
				e.preventDefault();
				settle(true);
			}}
		>
			{#if current.message}<p class="msg">{current.message}</p>{/if}
			{#if current.details?.length}
				<ul>
					{#each current.details as d (d)}<li>{d}</li>{/each}
				</ul>
			{/if}
			{#if current.typeToConfirm}
				<label class="wbr-field">
					<span>Type <code>{current.typeToConfirm}</code> to confirm</span>
					<!-- svelte-ignore a11y_autofocus -->
					<input bind:value={typed} autocomplete="off" spellcheck="false" autofocus />
				</label>
			{/if}
		</form>
	{/if}
	{#snippet footer()}
		<button type="button" class="wbr-btn wbr-btn-ghost" onclick={() => settle(false)}>
			{current?.cancelLabel ?? 'Cancel'}
		</button>
		<button
			type="submit"
			form="confirm-form"
			class={current?.danger ? 'wbr-btn-danger solid' : 'wbr-btn'}
			disabled={!canConfirm}
		>
			{current?.confirmLabel ?? 'Confirm'}
		</button>
	{/snippet}
</Modal>

<style>
	.body {
		display: grid;
		gap: 0.85rem;
	}

	.msg {
		margin: 0;
		color: var(--muted);
		font-size: 0.92rem;
		white-space: pre-wrap;
	}

	ul {
		margin: 0;
		padding-left: 1.1rem;
		color: var(--muted);
		font-size: 0.9rem;
		display: grid;
		gap: 0.2rem;
	}
</style>
