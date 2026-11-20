<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	type Site = {
		name: string;
		type: string;
		root?: string;
	};

	let sites = $state<Site[]>([]);
	let error = $state('');
	let loading = $state(true);
	let name = $state('');
	let siteKind = $state<'static' | 'app'>('static');
	let submitting = $state(false);
	let removing = $state('');

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await fetch('/api/sites', { credentials: 'same-origin' });
			if (res.status === 401) {
				await goto('/login');
				return;
			}
			const data = (await res.json()) as { ok?: boolean; sites?: Site[]; error?: string };
			if (!res.ok || !data.ok) {
				error = data.error || 'Failed to load sites';
				return;
			}
			sites = data.sites || [];
		} catch {
			error = 'Could not reach the panel API';
		} finally {
			loading = false;
		}
	}

	function isValidHostname(domain: string): boolean {
		return /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$/.test(domain);
	}

	async function addSite(e: Event) {
		e.preventDefault();
		error = '';
		const domain = name.trim().toLowerCase();
		name = domain;
		if (!isValidHostname(domain)) {
			error =
				'Enter a valid domain hostname (lowercase letters, digits, hyphens, and at least one dot), e.g. example.com';
			return;
		}
		submitting = true;
		try {
			const res = await fetch('/api/sites', {
				method: 'POST',
				credentials: 'same-origin',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name: domain, type: siteKind })
			});
			const data = (await res.json().catch(() => ({}))) as {
				ok?: boolean;
				error?: string;
			};
			if (!res.ok || !data.ok) {
				error = data.error || 'Could not add site';
				return;
			}
			await goto(`/sites/${encodeURIComponent(domain)}`);
		} catch {
			error = 'Could not reach the panel API';
		} finally {
			submitting = false;
		}
	}

	async function removeSite(siteName: string) {
		error = '';
		removing = siteName;
		try {
			const res = await fetch(`/api/sites/${encodeURIComponent(siteName)}`, {
				method: 'DELETE',
				credentials: 'same-origin'
			});
			const data = (await res.json().catch(() => ({}))) as { ok?: boolean; error?: string };
			if (!res.ok || !data.ok) {
				error = data.error || 'Could not remove site';
				return;
			}
			await load();
		} finally {
			removing = '';
		}
	}

	onMount(load);
</script>

<svelte:head>
	<title>Sites · Webooza</title>
</svelte:head>

<section class="page">
	<header class="page-head">
		<h1>Sites</h1>
		<p class="wbz-muted">Static and app sites on this VPS. Open a site to upload, deploy, or check DNS.</p>
	</header>

	{#if error}
		<p class="wbz-error" role="alert">{error}</p>
	{/if}

	<div class="layout">
		<section class="wbz-panel inventory" aria-labelledby="inventory-heading">
			<h2 id="inventory-heading">Your sites</h2>
			{#if loading}
				<p class="wbz-muted">Loading…</p>
			{:else if sites.length === 0}
				<p class="wbz-muted empty">No sites yet. Add a domain to get started.</p>
			{:else}
				<table>
					<thead>
						<tr>
							<th>Domain</th>
							<th>Type</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						{#each sites as site (site.name)}
							<tr>
								<td>
									<a class="site-link" href={`/sites/${encodeURIComponent(site.name)}`}>
										{site.name}
									</a>
								</td>
								<td>
									<span class="wbz-badge">{site.type}</span>
								</td>
								<td class="actions">
									<a class="open" href={`/sites/${encodeURIComponent(site.name)}`}>Open</a>
									<button
										type="button"
										class="wbz-btn-danger"
										disabled={removing === site.name}
										onclick={() => removeSite(site.name)}
									>
										{removing === site.name ? 'Removing…' : 'Remove'}
									</button>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</section>

		<form class="wbz-panel add" onsubmit={addSite} aria-labelledby="add-heading">
			<h2 id="add-heading">Add site</h2>
			<p class="lede">Creates the site user, directories, and Caddy route.</p>
			<label class="wbz-field">
				<span>Domain</span>
				<input name="name" bind:value={name} placeholder="example.com" required autocomplete="off" />
			</label>
			<label class="wbz-field">
				<span>Type</span>
				<select name="type" bind:value={siteKind}>
					<option value="static">Static</option>
					<option value="app">App</option>
				</select>
			</label>
			<button class="wbz-btn" type="submit" disabled={submitting}>
				{submitting ? 'Adding…' : 'Add site'}
			</button>
		</form>
	</div>
</section>

<style>
	.page {
		max-width: 56rem;
		display: grid;
		gap: 1.25rem;
	}

	.page-head h1 {
		margin: 0 0 0.35rem;
		font-size: clamp(1.85rem, 4vw, 2.35rem);
		letter-spacing: -0.03em;
		font-weight: 800;
	}

	.page-head p {
		margin: 0;
		max-width: 42ch;
	}

	.layout {
		display: grid;
		gap: 1.15rem;
		grid-template-columns: minmax(0, 1.4fr) minmax(16rem, 0.9fr);
		align-items: start;
	}

	.inventory {
		min-width: 0;
	}

	.empty {
		margin: 0.25rem 0 0;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.95rem;
	}

	th,
	td {
		text-align: left;
		padding: 0.7rem 0.35rem;
		border-bottom: 1px solid var(--border);
		vertical-align: middle;
	}

	th {
		font-size: 0.78rem;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--muted);
		font-weight: 800;
	}

	.site-link {
		font-weight: 800;
		text-decoration: none;
		color: var(--text);
	}

	.site-link:hover {
		color: var(--wbz-ultramarine);
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.55rem;
		justify-content: flex-end;
		align-items: center;
	}

	.open {
		font-weight: 800;
		font-size: 0.9rem;
		text-decoration: none;
		color: var(--wbz-ultramarine);
	}

	.add {
		display: grid;
		gap: 0.9rem;
	}

	.add .wbz-btn {
		justify-self: start;
		margin-top: 0.25rem;
	}

	@media (max-width: 820px) {
		.layout {
			grid-template-columns: 1fr;
		}

		.actions {
			justify-content: flex-start;
		}
	}
</style>
