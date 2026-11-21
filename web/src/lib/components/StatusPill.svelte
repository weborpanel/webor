<script lang="ts">
	type Tone = 'ok' | 'warn' | 'danger' | 'neutral' | 'info';

	type Props = {
		tone?: Tone;
		label: string;
		pulse?: boolean;
	};

	let { tone = 'neutral', label, pulse = false }: Props = $props();
</script>

<span class={['pill', tone]}>
	<span class={['dot', pulse && 'pulse']} aria-hidden="true"></span>
	{label}
</span>

<style>
	.pill {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		padding: 0.14rem 0.55rem 0.14rem 0.45rem;
		border-radius: 999px;
		font-size: 0.76rem;
		font-weight: 700;
		white-space: nowrap;
		background: var(--surface-3);
		color: var(--muted);
	}

	.dot {
		width: 0.48rem;
		height: 0.48rem;
		border-radius: 50%;
		background: currentColor;
	}

	.ok {
		background: var(--ok-soft);
		color: var(--ok);
	}

	.warn {
		background: var(--warn-soft);
		color: var(--warn);
	}

	.danger {
		background: var(--danger-soft);
		color: var(--danger);
	}

	.info {
		background: var(--info-soft);
		color: var(--info);
	}

	.pulse {
		box-shadow: 0 0 0 0 currentColor;
		animation: pulse 2s infinite;
	}

	@keyframes pulse {
		0% {
			box-shadow: 0 0 0 0 color-mix(in srgb, currentColor 60%, transparent);
		}
		70% {
			box-shadow: 0 0 0 6px transparent;
		}
		100% {
			box-shadow: 0 0 0 0 transparent;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.pulse {
			animation: none;
		}
	}
</style>
