<script lang="ts">
	import Icon from './Icon.svelte';
	import { toasts } from '#lib/stores/toasts.svelte.ts';

	type Props = {
		value: string;
		label?: string;
		/** Mask the value until revealed (secrets). */
		secret?: boolean;
	};

	let { value, label, secret = false }: Props = $props();

	let copied = $state(false);
	let revealed = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;

	/** Clipboard API needs a secure context (HTTPS/localhost); fall back for plain HTTP panels. */
	function fallbackCopy(text: string) {
		const ta = document.createElement('textarea');
		ta.value = text;
		ta.setAttribute('readonly', '');
		ta.setAttribute('aria-hidden', 'true');
		ta.style.cssText = 'position:fixed;left:-9999px;top:0;opacity:0;';
		document.body.appendChild(ta);
		ta.focus();
		ta.select();
		ta.setSelectionRange(0, text.length);
		let ok = false;
		try {
			ok = document.execCommand('copy');
		} finally {
			document.body.removeChild(ta);
		}
		if (!ok) throw new Error('execCommand copy failed');
	}

	async function copyText(text: string) {
		if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText && window.isSecureContext) {
			try {
				await navigator.clipboard.writeText(text);
				return;
			} catch {
				// fall through — Permissions-Policy or transient denial
			}
		}
		fallbackCopy(text);
	}

	async function copy() {
		const text = value ?? '';
		if (!text) {
			toasts.error('Nothing to copy');
			return;
		}
		try {
			await copyText(text);
			copied = true;
			clearTimeout(timer);
			timer = setTimeout(() => (copied = false), 1500);
		} catch {
			toasts.error('Could not copy to clipboard');
		}
	}
</script>

<span class="copy-field">
	{#if label}<span class="label">{label}</span>{/if}
	<code class="value">{secret && !revealed ? '•'.repeat(Math.min(value.length, 24)) : value}</code>
	{#if secret}
		<button type="button" class="btn" onclick={() => (revealed = !revealed)}>
			{revealed ? 'Hide' : 'Show'}
		</button>
	{/if}
	<button type="button" class="btn" onclick={copy} aria-label={copied ? 'Copied' : `Copy ${label ?? 'value'}`}>
		<Icon name={copied ? 'check' : 'copy'} size={14} />
		<span>{copied ? 'Copied' : 'Copy'}</span>
	</button>
</span>

<style>
	.copy-field {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		min-width: 0;
		padding: 0.3rem 0.35rem 0.3rem 0.6rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-2);
	}

	.label {
		font-size: 0.75rem;
		font-weight: 800;
		color: var(--subtle);
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.value {
		flex: 1;
		min-width: 0;
		background: none;
		padding: 0;
		overflow-wrap: anywhere;
		font-size: 0.85rem;
	}

	.btn {
		display: inline-flex;
		align-items: center;
		gap: 0.3rem;
		flex-shrink: 0;
		font: inherit;
		font-size: 0.78rem;
		font-weight: 700;
		padding: 0.25rem 0.5rem;
		border-radius: 6px;
		border: 1px solid var(--border-strong);
		background: var(--surface);
		color: var(--text);
		cursor: pointer;
	}

	.btn:hover {
		background: var(--surface-3);
	}
</style>
