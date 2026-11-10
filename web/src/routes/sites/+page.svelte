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
	let submitting = $state(false);

	let selected = $state('');
	let uploadPath = $state('index.html');
	let uploadFile = $state<FileList | null>(null);
	let uploading = $state(false);

	type DnsItem = {
		record: string;
		host: string;
		want?: string;
		got?: string;
		status: string;
		detail?: string;
	};
	let dnsItems = $state<DnsItem[]>([]);
	let dnsOK = $state<boolean | null>(null);
	let dnsCanCert = $state(false);
	let dnsLoading = $state(false);
	let expectedA = $state('');

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
			if (!selected && sites.length) selected = sites[0].name;
		} catch {
			error = 'Could not reach the panel API';
		} finally {
			loading = false;
		}
	}

	async function addSite(e: Event) {
		e.preventDefault();
		submitting = true;
		error = '';
		try {
			const res = await fetch('/api/sites', {
				method: 'POST',
				credentials: 'same-origin',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name, type: 'static' })
			});
			const data = (await res.json().catch(() => ({}))) as {
				ok?: boolean;
				error?: string;
				warning?: string;
			};
			if (!res.ok || !data.ok) {
				error = data.error || 'Could not add site';
				return;
			}
			if (data.warning) error = data.warning;
			selected = name;
			name = '';
			await load();
		} catch {
			error = 'Could not reach the panel API';
		} finally {
			submitting = false;
		}
	}

	async function removeSite(siteName: string) {
		error = '';
		const res = await fetch(`/api/sites/${encodeURIComponent(siteName)}`, {
			method: 'DELETE',
			credentials: 'same-origin'
		});
		const data = (await res.json().catch(() => ({}))) as { ok?: boolean; error?: string };
		if (!res.ok || !data.ok) {
			error = data.error || 'Could not remove site';
			return;
		}
		if (selected === siteName) selected = '';
		await load();
	}

	async function upload(e: Event) {
		e.preventDefault();
		if (!selected || !uploadFile?.length) {
			error = 'Choose a site and a file';
			return;
		}
		uploading = true;
		error = '';
		try {
			const file = uploadFile[0];
			const path = uploadPath.trim() || file.name;
			const res = await fetch(
				`/api/sites/${encodeURIComponent(selected)}/files/${path.split('/').map(encodeURIComponent).join('/')}`,
				{
					method: 'PUT',
					credentials: 'same-origin',
					headers: { 'Content-Type': 'application/octet-stream' },
					body: await file.arrayBuffer()
				}
			);
			const data = (await res.json().catch(() => ({}))) as { ok?: boolean; error?: string };
			if (!res.ok || !data.ok) {
				error = data.error || 'Upload failed';
				return;
			}
			uploadFile = null;
		} catch {
			error = 'Could not reach the panel API';
		} finally {
			uploading = false;
		}
	}

	async function checkDNS() {
		if (!selected) return;
		dnsLoading = true;
		error = '';
		try {
			const q = expectedA.trim() ? `?a=${encodeURIComponent(expectedA.trim())}` : '';
			const res = await fetch(`/api/sites/${encodeURIComponent(selected)}/dns${q}`, {
				credentials: 'same-origin'
			});
			const data = (await res.json()) as {
				ok?: boolean;
				error?: string;
				can_issue_certificate?: boolean;
				reports?: { ok: boolean; items: DnsItem[] }[];
			};
			if (!res.ok || !data.ok) {
				error = data.error || 'DNS check failed';
				return;
			}
			dnsItems = data.reports?.[0]?.items || [];
			dnsOK = data.reports?.[0]?.ok ?? null;
			dnsCanCert = !!data.can_issue_certificate;
		} catch {
			error = 'Could not reach the panel API';
		} finally {
			dnsLoading = false;
		}
	}

	onMount(load);
</script>

<svelte:head>
	<title>Sites · Webooza</title>
</svelte:head>

<section class="sites">
	<header>
		<h1>Sites</h1>
		<p>Each site gets a <code>public/</code> folder (like public_html). Upload HTML there; Caddy serves it.</p>
	</header>

	{#if error}
		<p class="error" role="alert">{error}</p>
	{/if}

	{#if loading}
		<p class="muted">Loading…</p>
	{:else if sites.length === 0}
		<p class="muted">No sites yet. Add a domain below.</p>
	{:else}
		<ul>
			{#each sites as site (site.name)}
				<li>
					<button
						type="button"
						class={['pick', selected === site.name && 'active']}
						onclick={() => (selected = site.name)}
					>
						<strong>{site.name}</strong>
						<span class="meta">{site.root || 'public'}/ · <a href={`/sites/${encodeURIComponent(site.name)}`} onclick={(e) => e.stopPropagation()}>dashboard</a></span>
					</button>
					<button type="button" class="danger" onclick={() => removeSite(site.name)}>Remove</button>
				</li>
			{/each}
		</ul>
	{/if}

	<form onsubmit={addSite}>
		<h2>Add site</h2>
		<label>
			<span>Domain</span>
			<input name="name" bind:value={name} placeholder="example.com" required />
		</label>
		<button type="submit" disabled={submitting}>{submitting ? 'Adding…' : 'Add site'}</button>
	</form>

	{#if selected}
		<form class="upload" onsubmit={upload}>
			<h2>Upload into {selected}/public/</h2>
			<label>
				<span>Path under public/</span>
				<input name="path" bind:value={uploadPath} placeholder="index.html" required />
			</label>
			<label>
				<span>File</span>
				<input
					type="file"
					onchange={(e) => {
						uploadFile = e.currentTarget.files;
					}}
				/>
			</label>
			<button type="submit" disabled={uploading}>{uploading ? 'Uploading…' : 'Upload'}</button>
		</form>

		<div class="dns">
			<h2>DNS checklist · {selected}</h2>
			<p class="muted">Point the domain at this VPS, then run the check. Certificate issuance waits for a green A record.</p>
			<label>
				<span>Expected A (optional override)</span>
				<input bind:value={expectedA} placeholder="203.0.113.10" />
			</label>
			<button type="button" disabled={dnsLoading} onclick={checkDNS}>
				{dnsLoading ? 'Checking…' : 'Check DNS'}
			</button>
			{#if dnsOK !== null}
				<p class={['dns-summary', dnsCanCert ? 'good' : 'bad']}>
					{dnsCanCert ? 'Ready for certificate issuance' : 'Not ready — fix failing records first'}
				</p>
				<table>
					<thead>
						<tr><th>Record</th><th>Status</th><th>Want</th><th>Got</th></tr>
					</thead>
					<tbody>
						{#each dnsItems as item, i (item.record + item.host + i)}
							<tr>
								<td>{item.record}</td>
								<td class={item.status}>{item.status}</td>
								<td>{item.want || '—'}</td>
								<td>{item.got || item.detail || '—'}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{/if}
</section>

<style>
	.sites {
		max-width: 40rem;
		padding: 1rem 0 4rem;
	}

	header {
		margin-bottom: 1.5rem;
	}

	h1 {
		margin: 0 0 0.4rem;
		font-size: clamp(2rem, 5vw, 2.6rem);
		letter-spacing: -0.03em;
		font-weight: 800;
	}

	h2 {
		margin: 0 0 0.75rem;
		font-size: 1.15rem;
	}

	p {
		margin: 0;
		color: var(--muted);
	}

	code {
		font-size: 0.92em;
	}

	.muted {
		margin: 0 0 1.25rem;
		color: var(--muted);
	}

	.error {
		margin: 0 0 1rem;
		color: #b42318;
		font-weight: 600;
	}

	ul {
		list-style: none;
		margin: 0 0 2rem;
		padding: 0;
		display: grid;
		gap: 0.65rem;
	}

	li {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 0.35rem 0;
		border-bottom: 1px solid color-mix(in srgb, var(--text) 12%, transparent);
	}

	.pick {
		display: grid;
		gap: 0.15rem;
		text-align: left;
		background: transparent;
		border: 0;
		padding: 0.5rem 0;
		color: inherit;
		font: inherit;
		cursor: pointer;
	}

	.pick.active strong {
		color: var(--wbz-ultramarine);
	}

	.meta {
		font-size: 0.9rem;
		color: var(--muted);
		font-weight: 500;
	}

	form {
		display: grid;
		gap: 0.9rem;
		max-width: 26rem;
		margin-bottom: 2rem;
	}

	label {
		display: grid;
		gap: 0.35rem;
		font-weight: 700;
		font-size: 0.92rem;
	}

	input {
		font: inherit;
		padding: 0.75rem 0.9rem;
		border-radius: 12px;
		border: 1px solid color-mix(in srgb, var(--text) 16%, transparent);
		background: color-mix(in srgb, var(--surface) 92%, transparent);
		color: var(--text);
	}

	button {
		font: inherit;
		font-weight: 800;
		padding: 0.75rem 1.1rem;
		border: 0;
		border-radius: 999px;
		background: var(--wbz-ultramarine);
		color: #fff;
		cursor: pointer;
		justify-self: start;
	}

	button:disabled {
		opacity: 0.7;
		cursor: wait;
	}

	button.danger {
		background: transparent;
		color: #b42318;
		border: 1px solid color-mix(in srgb, #b42318 35%, transparent);
		padding: 0.45rem 0.85rem;
		font-size: 0.85rem;
	}

	.dns {
		display: grid;
		gap: 0.75rem;
		max-width: 36rem;
	}

	.dns-summary {
		font-weight: 700;
	}

	.dns-summary.good {
		color: #067647;
	}

	.dns-summary.bad {
		color: #b42318;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.9rem;
	}

	th,
	td {
		text-align: left;
		padding: 0.45rem 0.35rem;
		border-bottom: 1px solid color-mix(in srgb, var(--text) 12%, transparent);
		vertical-align: top;
	}

	td.pass {
		color: #067647;
		font-weight: 700;
	}

	td.fail {
		color: #b42318;
		font-weight: 700;
	}

	td.warn,
	td.skip {
		color: #b54708;
		font-weight: 700;
	}
</style>
