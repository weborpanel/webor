<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import Icon from './Icon.svelte';
	import type { IconName } from '#lib/icons/paths.ts';
	import { overview, recentSites } from '#lib/stores/overview.svelte.ts';
	import { siteFromPath, siteSections } from '#lib/siteNav.ts';
	import { theme } from '#lib/stores/theme.svelte.ts';

	type Props = {
		open: boolean;
		onaddsite: () => void;
	};

	let { open = $bindable(), onaddsite }: Props = $props();

	type Command = {
		id: string;
		label: string;
		hint?: string;
		group: string;
		icon: IconName;
		run: () => void;
	};

	let query = $state('');
	let index = $state(0);

	function nav(href: string) {
		return () => void goto(href);
	}

	let commands = $derived.by((): Command[] => {
		const out: Command[] = [
			{ id: 'nav:dash', label: 'Dashboard', group: 'Navigate', icon: 'dashboard', run: nav('/dashboard') },
			{ id: 'nav:sites', label: 'All sites', group: 'Navigate', icon: 'globe', run: nav('/sites') },
			{ id: 'nav:activity', label: 'Activity log', group: 'Navigate', icon: 'activity', run: nav('/activity') },
			{ id: 'nav:account', label: 'Account & security', group: 'Navigate', icon: 'user', run: nav('/account') },
			{
				id: 'nav:db',
				label: 'Open database browser',
				group: 'Navigate',
				icon: 'database',
				run: () => window.open('/db/', '_blank', 'noopener')
			},
			{ id: 'act:add', label: 'Add site', group: 'Actions', icon: 'plus', run: onaddsite },
			{
				id: 'act:theme',
				label: 'Toggle theme',
				hint: theme.pref,
				group: 'Actions',
				icon: 'moon',
				run: () => theme.cycle()
			}
		];
		const current = siteFromPath(page.url.pathname).toLowerCase();
		const recent = recentSites.names;
		const sites = [...overview.sites].sort((a, b) => {
			const ra = recent.indexOf(a.name.toLowerCase());
			const rb = recent.indexOf(b.name.toLowerCase());
			return (ra < 0 ? 99 : ra) - (rb < 0 ? 99 : rb) || a.name.localeCompare(b.name);
		});
		for (const s of sites) {
			out.push({
				id: `site:${s.name}`,
				label: s.name,
				hint: s.type,
				group: 'Sites',
				icon: 'globe',
				run: nav(`/sites/${encodeURIComponent(s.name)}`)
			});
		}
		const q = query.trim().toLowerCase();
		const sectionSites = q
			? sites.filter((s) => q.includes(s.name.toLowerCase()) || s.name.toLowerCase().startsWith(q.split(' ')[0]))
			: sites.filter((s) => s.name.toLowerCase() === current);
		for (const s of sectionSites.slice(0, 3)) {
			for (const sec of siteSections(s.name, s.type).slice(1)) {
				out.push({
					id: `sec:${s.name}:${sec.key}`,
					label: `${s.name} › ${sec.label}`,
					group: 'Site sections',
					icon: sec.icon ?? 'chevronRight',
					run: nav(sec.href)
				});
			}
		}
		return out;
	});

	let results = $derived.by(() => {
		const terms = query.trim().toLowerCase().split(/\s+/).filter(Boolean);
		if (!terms.length) return commands.slice(0, 40);
		return commands
			.filter((c) => {
				const hay = `${c.label} ${c.group} ${c.hint ?? ''}`.toLowerCase();
				return terms.every((t) => hay.includes(t));
			})
			.slice(0, 40);
	});

	let selected = $derived(Math.min(index, Math.max(results.length - 1, 0)));

	function run(c: Command | undefined) {
		if (!c) return;
		open = false;
		query = '';
		index = 0;
		c.run();
	}

	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			index = (selected + 1) % Math.max(results.length, 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			index = (selected - 1 + results.length) % Math.max(results.length, 1);
		} else if (e.key === 'Enter') {
			e.preventDefault();
			run(results[selected]);
		}
	}

	function sync(dialog: HTMLDialogElement) {
		if (open && !dialog.open) {
			dialog.showModal();
			dialog.querySelector('input')?.focus();
		} else if (!open && dialog.open) {
			dialog.close();
		}
	}

	function scrollSelected(node: HTMLElement, isSelected: boolean) {
		if (isSelected) node.scrollIntoView({ block: 'nearest' });
	}
</script>

<dialog
	{@attach sync}
	class="palette"
	aria-label="Command palette"
	onclose={() => {
		open = false;
		query = '';
		index = 0;
	}}
	onclick={(e) => {
		if (e.target === e.currentTarget) open = false;
	}}
>
	{#if open}
		<div class="panel">
			<label class="input-row">
				<Icon name="search" size={17} />
				<span class="visually-hidden">Search commands</span>
				<input
					bind:value={() => query, (v) => {
						query = v;
						index = 0;
					}}
					placeholder="Search sites, pages and actions…"
					autocomplete="off"
					spellcheck="false"
					role="combobox"
					aria-expanded="true"
					aria-controls="palette-list"
					aria-activedescendant={results[selected] ? `cmd-${selected}` : undefined}
					{onkeydown}
				/>
				<kbd>Esc</kbd>
			</label>
			<ul id="palette-list" role="listbox">
				{#each results as c, i (c.id)}
					{#if i === 0 || results[i - 1].group !== c.group}
						<li class="group" role="presentation">{c.group}</li>
					{/if}
					<li
						id={`cmd-${i}`}
						role="option"
						aria-selected={i === selected}
						class={['item', i === selected && 'selected']}
						{@attach (node) => scrollSelected(node, i === selected)}
						onclick={() => run(c)}
						onkeydown={() => {}}
						onmousemove={() => (index = i)}
					>
						<Icon name={c.icon} size={15} />
						<span class="label">{c.label}</span>
						{#if c.hint}<span class="hint">{c.hint}</span>{/if}
					</li>
				{:else}
					<li class="none">No results for “{query}”</li>
				{/each}
			</ul>
		</div>
	{/if}
</dialog>

<style>
	.palette {
		padding: 0;
		border: 0;
		background: transparent;
		width: min(36rem, calc(100vw - 2rem));
		margin-top: 12vh;
		color: var(--text);
	}

	.palette::backdrop {
		background: rgb(8 11 30 / 0.5);
		backdrop-filter: blur(2px);
	}

	.panel {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-lg);
		overflow: hidden;
	}

	.input-row {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		padding: 0.85rem 1rem;
		border-bottom: 1px solid var(--border);
		color: var(--subtle);
	}

	input {
		flex: 1;
		min-width: 0;
		border: 0;
		outline: none;
		background: none;
		font: inherit;
		font-size: 1rem;
		color: var(--text);
	}

	ul {
		list-style: none;
		margin: 0;
		padding: 0.4rem;
		max-height: min(24rem, 60vh);
		overflow-y: auto;
	}

	.group {
		padding: 0.55rem 0.6rem 0.25rem;
		font-size: 0.7rem;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--subtle);
	}

	.item {
		display: flex;
		align-items: center;
		gap: 0.65rem;
		padding: 0.55rem 0.65rem;
		border-radius: var(--radius-sm);
		cursor: pointer;
		font-size: 0.9rem;
		font-weight: 600;
		color: var(--muted);
	}

	.item.selected {
		background: var(--accent-soft);
		color: var(--accent);
	}

	.label {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text);
	}

	.hint {
		font-size: 0.75rem;
		color: var(--subtle);
	}

	.none {
		padding: 1.25rem;
		text-align: center;
		color: var(--muted);
		font-size: 0.9rem;
	}
</style>
