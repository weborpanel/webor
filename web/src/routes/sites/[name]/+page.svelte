<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';

	type DnsItem = {
		record: string;
		host: string;
		want?: string;
		got?: string;
		status: string;
		detail?: string;
	};

	let name = $derived(page.params.name || '');
	let error = $state('');
	let loading = $state(true);
	let exists = $state(false);
	let root = $state('public');
	let dnsItems = $state<DnsItem[]>([]);
	let dnsCanCert = $state(false);
	let dnsChecked = $state(false);
	let dnsLoading = $state(false);

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await fetch('/api/sites', { credentials: 'same-origin' });
			if (res.status === 401) {
				await goto('/login');
				return;
			}
			const data = (await res.json()) as {
				ok?: boolean;
				error?: string;
				sites?: { name: string; root?: string }[];
			};
			if (!res.ok || !data.ok) {
				error = data.error || 'Failed to load site';
				return;
			}
			const site = (data.sites || []).find((s) => s.name.toLowerCase() === name.toLowerCase());
			exists = !!site;
			root = site?.root || 'public';
		} catch {
			error = 'Could not reach the panel API';
		} finally {
			loading = false;
		}
	}

	async function checkDNS() {
		dnsLoading = true;
		error = '';
		try {
			const res = await fetch(`/api/sites/${encodeURIComponent(name)}/dns`, {
				credentials: 'same-origin'
			});
			const data = (await res.json()) as {
				ok?: boolean;
				error?: string;
				can_issue_certificate?: boolean;
				reports?: { items: DnsItem[] }[];
			};
			if (!res.ok || !data.ok) {
				error = data.error || 'DNS check failed';
				return;
			}
			dnsItems = data.reports?.[0]?.items || [];
			dnsCanCert = !!data.can_issue_certificate;
			dnsChecked = true;
		} catch {
			error = 'Could not reach the panel API';
		} finally {
			dnsLoading = false;
		}
	}

	onMount(load);
</script>

<svelte:head>
	<title>{name} · Webooza</title>
</svelte:head>

<section class="dash">
	<p class="back"><a href="/sites">← Sites</a></p>
	{#if loading}
		<p class="muted">Loading…</p>
	{:else if !exists}
		<p class="error" role="alert">Site not found in config.</p>
	{:else}
		<header>
			<h1>{name}</h1>
			<p>Document root <code>/srv/sites/{name}/{root}/</code></p>
		</header>

		{#if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		<section class="next">
			<h2>Next steps</h2>
			<ol>
				<li>
					<strong>Upload files</strong> into <code>{root}/</code> —
					<a href="/sites">open the Sites page</a> and use the upload form for this domain.
				</li>
				<li>
					<strong>Run DNS checklist</strong> — point the domain at this VPS, then check below.
				</li>
				<li>
					<strong>Certificate gating</strong> —
					{#if !dnsChecked}
						run the DNS check to see whether HTTPS can be issued.
					{:else if dnsCanCert}
						DNS is ready for certificate issuance.
					{:else}
						DNS is not ready for HTTPS yet — fix failing records first.
					{/if}
				</li>
			</ol>
		</section>

		<div class="actions">
			<a class="button" href="/sites">Upload files on Sites</a>
			<button type="button" disabled={dnsLoading} onclick={checkDNS}>
				{dnsLoading ? 'Checking DNS…' : 'Check DNS'}
			</button>
		</div>

		{#if dnsChecked}
			<p class={['summary', dnsCanCert ? 'good' : 'bad']}>
				{dnsCanCert ? 'DNS ready for certificate issuance' : 'DNS not ready for HTTPS yet'}
			</p>
			<table>
				<thead>
					<tr><th>Record</th><th>Status</th><th>Detail</th></tr>
				</thead>
				<tbody>
					{#each dnsItems as item, i (item.record + item.host + i)}
						<tr>
							<td>{item.record}</td>
							<td class={item.status}>{item.status}</td>
							<td>{item.got || item.detail || item.want || '—'}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	{/if}
</section>

<style>
	.dash {
		max-width: 40rem;
		padding: 1rem 0 4rem;
	}

	.back {
		margin: 0 0 1rem;
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

	header p {
		margin: 0;
		color: var(--muted);
	}

	.muted {
		color: var(--muted);
	}

	.error {
		color: #b42318;
		font-weight: 600;
	}

	.next {
		margin: 1.5rem 0 0;
	}

	ol {
		margin: 0;
		padding-left: 1.25rem;
		display: grid;
		gap: 0.65rem;
		color: var(--muted);
	}

	ol strong {
		color: var(--text);
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
		margin: 1.5rem 0;
	}

	.button,
	button {
		font: inherit;
		font-weight: 800;
		padding: 0.75rem 1.1rem;
		border: 0;
		border-radius: 999px;
		background: var(--wbz-ultramarine);
		color: #fff;
		cursor: pointer;
		text-decoration: none;
		display: inline-flex;
		align-items: center;
	}

	button:disabled {
		opacity: 0.7;
		cursor: wait;
	}

	.summary {
		font-weight: 700;
		margin: 0 0 0.75rem;
	}

	.summary.good {
		color: #067647;
	}

	.summary.bad {
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
