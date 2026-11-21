<script lang="ts">
	import Icon from './Icon.svelte';
	import { formatBytes } from '#lib/format.ts';

	type Props = {
		file: File | null;
		label: string;
		hint?: string;
		disabled?: boolean;
		accept?: string;
		onselect?: (file: File | null) => void;
	};

	let { file = $bindable(), label, hint, disabled = false, accept, onselect }: Props = $props();

	let dragging = $state(false);
	let input: HTMLInputElement | undefined = $state();

	function pick(f: File | null) {
		file = f;
		onselect?.(f);
	}
</script>

<label
	class={['drop', dragging && 'dragging', file && 'has-file', disabled && 'disabled']}
	ondragover={(e) => {
		e.preventDefault();
		if (!disabled) dragging = true;
	}}
	ondragleave={() => (dragging = false)}
	ondrop={(e) => {
		e.preventDefault();
		dragging = false;
		if (disabled) return;
		pick(e.dataTransfer?.files?.[0] ?? null);
	}}
>
	<input
		bind:this={input}
		type="file"
		{accept}
		{disabled}
		onchange={(e) => pick(e.currentTarget.files?.[0] ?? null)}
	/>
	<span class="icon"><Icon name={file ? 'file' : 'upload'} size={20} /></span>
	{#if file}
		<span class="text">
			<strong>{file.name}</strong>
			<span>{formatBytes(file.size)}</span>
		</span>
		<button
			type="button"
			class="clear"
			aria-label="Remove file"
			{disabled}
			onclick={(e) => {
				e.preventDefault();
				if (input) input.value = '';
				pick(null);
			}}
		>
			<Icon name="x" size={14} />
		</button>
	{:else}
		<span class="text">
			<strong>{label}</strong>
			<span>{hint ?? 'Drag a file here or click to browse'}</span>
		</span>
	{/if}
</label>

<style>
	.drop {
		position: relative;
		display: flex;
		align-items: center;
		gap: 0.85rem;
		padding: 1rem 1.1rem;
		border: 1.5px dashed var(--border-strong);
		border-radius: var(--radius);
		background: var(--surface-2);
		cursor: pointer;
		transition:
			border-color 0.12s,
			background 0.12s;
	}

	.drop:hover,
	.dragging {
		border-color: var(--accent);
		background: var(--accent-soft);
	}

	.drop:has(input:focus-visible) {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}

	.has-file {
		border-style: solid;
	}

	.disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	input {
		position: absolute;
		opacity: 0;
		width: 1px;
		height: 1px;
		pointer-events: none;
	}

	.icon {
		display: grid;
		place-items: center;
		width: 2.4rem;
		height: 2.4rem;
		border-radius: 10px;
		background: var(--surface);
		border: 1px solid var(--border);
		color: var(--accent);
		flex-shrink: 0;
	}

	.text {
		display: grid;
		flex: 1;
		min-width: 0;
	}

	.text strong {
		font-size: 0.9rem;
		overflow-wrap: anywhere;
	}

	.text span {
		font-size: 0.8rem;
		color: var(--muted);
	}

	.clear {
		background: none;
		border: 0;
		color: var(--subtle);
		cursor: pointer;
		padding: 0.3rem;
		border-radius: 6px;
	}

	.clear:hover {
		background: var(--surface-3);
		color: var(--text);
	}
</style>
