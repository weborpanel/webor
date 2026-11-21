<script lang="ts">
	import CopyField from './CopyField.svelte';
	import Icon from './Icon.svelte';

	type Props = {
		subject: string;
		secret: string;
		rotated?: boolean;
		ondone: () => void;
	};

	let { subject, secret, rotated = false, ondone }: Props = $props();
</script>

<div class="secret" role="status">
	<div class="head">
		<Icon name="key" size={16} />
		<p>
			{rotated ? 'New password' : 'Password'} for <code>{subject}</code>. Copy it now; it won't be
			shown again.
		</p>
	</div>
	<CopyField value={secret} secret />
	<div>
		<button class="wbr-btn sm" type="button" onclick={ondone}>I've saved it</button>
	</div>
</div>

<style>
	.secret {
		display: grid;
		gap: 0.6rem;
		margin: 0 0 1rem;
		padding: 0.85rem 1rem;
		border-radius: var(--radius);
		border: 1px solid color-mix(in srgb, var(--accent) 35%, var(--border));
		background: var(--accent-soft);
	}

	.head {
		display: flex;
		gap: 0.5rem;
		align-items: flex-start;
		color: var(--accent);
	}

	.head p {
		margin: 0;
		font-size: 0.9rem;
		color: var(--text);
	}
</style>
