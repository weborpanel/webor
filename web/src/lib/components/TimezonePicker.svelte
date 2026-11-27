<script lang="ts">
	import { filterTimeZones } from '#lib/timezones.ts';

	type Props = {
		value?: string;
		disabled?: boolean;
		/** Shown on the empty “follow host” option. */
		hostLabel?: string;
		id?: string;
	};

	let {
		value = $bindable(''),
		disabled = false,
		hostLabel = 'Host OS timezone',
		id = 'timezone-picker'
	}: Props = $props();

	let open = $state(false);
	let query = $state('');
	let active = $state(0);
	let root: HTMLDivElement | undefined = $state();
	let inputEl: HTMLInputElement | undefined = $state();

	let options = $derived.by(() => {
		const zones = filterTimeZones(query, 100);
		return [{ value: '', label: hostLabel }, ...zones.map((z) => ({ value: z, label: z }))];
	});

	let display = $derived(value.trim() ? value.trim() : hostLabel);

	function openList() {
		if (disabled) return;
		open = true;
		query = value.trim();
		active = Math.max(
			0,
			options.findIndex((o) => o.value === value.trim())
		);
		queueMicrotask(() => inputEl?.select());
	}

	function closeList() {
		open = false;
		query = '';
		active = 0;
	}

	function pick(next: string) {
		value = next;
		closeList();
	}

	function onKeydown(e: KeyboardEvent) {
		if (!open) {
			if (e.key === 'ArrowDown' || e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				openList();
			}
			return;
		}
		if (e.key === 'Escape') {
			e.preventDefault();
			closeList();
			return;
		}
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			active = Math.min(options.length - 1, active + 1);
			return;
		}
		if (e.key === 'ArrowUp') {
			e.preventDefault();
			active = Math.max(0, active - 1);
			return;
		}
		if (e.key === 'Enter') {
			e.preventDefault();
			const opt = options[active];
			if (opt) pick(opt.value);
		}
	}

	function onDocPointer(e: PointerEvent) {
		if (!open || !root) return;
		if (e.target instanceof Node && !root.contains(e.target)) closeList();
	}

	$effect(() => {
		if (!open) return;
		document.addEventListener('pointerdown', onDocPointer);
		return () => document.removeEventListener('pointerdown', onDocPointer);
	});
</script>

<div class="picker" class:open class:disabled bind:this={root}>
	{#if open}
		<input
			{id}
			bind:this={inputEl}
			class="wbr-input search"
			type="search"
			role="combobox"
			aria-expanded="true"
			aria-controls="{id}-list"
			aria-autocomplete="list"
			aria-activedescendant={options[active] ? `${id}-opt-${active}` : undefined}
			placeholder="Search timezones…"
			bind:value={query}
			{disabled}
			autocomplete="off"
			spellcheck="false"
			onkeydown={onKeydown}
			oninput={() => (active = 0)}
		/>
	{:else}
		<button
			{id}
			type="button"
			class="trigger"
			aria-haspopup="listbox"
			aria-expanded="false"
			{disabled}
			onclick={openList}
			onkeydown={onKeydown}
		>
			<span class="value" class:empty={!value.trim()}>{display}</span>
			<span class="chev" aria-hidden="true">▾</span>
		</button>
	{/if}

	{#if open}
		<ul id="{id}-list" class="list" role="listbox" aria-label="IANA timezones">
			{#each options as opt, i (opt.value || '__host__')}
				<li
					id="{id}-opt-{i}"
					role="option"
					aria-selected={opt.value === value.trim()}
					class:active={i === active}
					class:host={!opt.value}
					onpointerdown={(e) => {
						e.preventDefault();
						pick(opt.value);
					}}
				>
					{opt.label}
				</li>
			{:else}
				<li class="none" role="presentation">No matches</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.picker {
		position: relative;
		width: min(28rem, 100%);
	}

	.trigger,
	.search {
		width: 100%;
		text-align: left;
	}

	.trigger {
		font: inherit;
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.55rem 0.75rem;
		border-radius: var(--radius-sm);
		border: 1px solid var(--border-strong);
		background: var(--surface);
		color: var(--text);
		cursor: pointer;
		font-size: 0.92rem;
		font-weight: 500;
	}

	.trigger:hover:not(:disabled) {
		border-color: var(--accent);
	}

	.trigger:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.value {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-family: var(--mono);
		font-size: 0.88rem;
	}

	.value.empty {
		font-family: var(--font);
		color: var(--muted);
		font-weight: 600;
	}

	.chev {
		color: var(--subtle);
		font-size: 0.75rem;
	}

	.list {
		position: absolute;
		z-index: 20;
		left: 0;
		right: 0;
		top: calc(100% + 0.35rem);
		margin: 0;
		padding: 0.35rem;
		list-style: none;
		max-height: min(18rem, 50vh);
		overflow: auto;
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		box-shadow: var(--shadow-lg);
	}

	.list li {
		border-radius: 6px;
		padding: 0.45rem 0.6rem;
		font-family: var(--mono);
		font-size: 0.84rem;
		cursor: pointer;
		color: var(--text);
	}

	.list li.host {
		font-family: var(--font);
		font-weight: 700;
		color: var(--muted);
		border-bottom: 1px solid var(--border);
		margin-bottom: 0.2rem;
		border-radius: 6px 6px 0 0;
	}

	.list li.active,
	.list li:hover {
		background: var(--accent-soft);
		color: var(--accent);
	}

	.list li.none {
		color: var(--muted);
		cursor: default;
		font-family: var(--font);
		text-align: center;
	}
</style>
