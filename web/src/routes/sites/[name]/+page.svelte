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

	type DeployMode = 'binary' | 'git';

	let name = $derived(page.params.name || '');
	let error = $state('');
	let loading = $state(true);
	let exists = $state(false);
	let root = $state('public');
	let siteType = $state('static');
	let dnsItems = $state<DnsItem[]>([]);
	let dnsCanCert = $state(false);
	let dnsChecked = $state(false);
	let dnsLoading = $state(false);
	let expectedA = $state('');

	let deployMode = $state<DeployMode>('binary');
	let deployBusy = $state(false);
	let deployError = $state('');
	let deploySuccess = $state('');
	let binaryFiles = $state<FileList | null>(null);
	let gitUrl = $state('');
	let gitRef = $state('');
	let buildCommand = $state('');
	let artifact = $state('');
	let isApp = $derived(siteType === 'app');

	let uploadPath = $state('index.html');
	let uploadFile = $state<FileList | null>(null);
	let uploading = $state(false);
	let uploadOk = $state('');

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
				sites?: { name: string; type?: string; root?: string }[];
			};
			if (!res.ok || !data.ok) {
				error = data.error || 'Failed to load site';
				return;
			}
			const site = (data.sites || []).find((s) => s.name.toLowerCase() === name.toLowerCase());
			exists = !!site;
			root = site?.root || 'public';
			siteType = site?.type || 'static';
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
			const q = expectedA.trim() ? `?a=${encodeURIComponent(expectedA.trim())}` : '';
			const res = await fetch(`/api/sites/${encodeURIComponent(name)}/dns${q}`, {
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

	async function upload(e: Event) {
		e.preventDefault();
		uploadOk = '';
		error = '';
		if (!uploadFile?.length) {
			error = 'Choose a file to upload';
			return;
		}
		uploading = true;
		try {
			const file = uploadFile[0];
			const path = uploadPath.trim() || file.name;
			const res = await fetch(
				`/api/sites/${encodeURIComponent(name)}/files/${path.split('/').map(encodeURIComponent).join('/')}`,
				{
					method: 'PUT',
					credentials: 'same-origin',
					headers: { 'Content-Type': 'application/octet-stream' },
					body: await file.arrayBuffer()
				}
			);
			if (res.status === 401) {
				await goto('/login');
				return;
			}
			const data = (await res.json().catch(() => ({}))) as { ok?: boolean; error?: string };
			if (!res.ok || !data.ok) {
				error = data.error || 'Upload failed';
				return;
			}
			uploadOk = `Uploaded to ${root}/${path}`;
			uploadFile = null;
		} catch {
			error = 'Could not reach the panel API';
		} finally {
			uploading = false;
		}
	}

	async function deployBinary(e: Event) {
		e.preventDefault();
		deployError = '';
		deploySuccess = '';
		if (!binaryFiles?.length) {
			deployError = 'Choose a binary file';
			return;
		}
		deployBusy = true;
		try {
			const body = new FormData();
			body.append('file', binaryFiles[0]);
			const res = await fetch(`/api/sites/${encodeURIComponent(name)}/deploy`, {
				method: 'POST',
				credentials: 'same-origin',
				body
			});
			if (res.status === 401) {
				await goto('/login');
				return;
			}
			const data = (await res.json().catch(() => ({}))) as {
				ok?: boolean;
				error?: string;
				release_id?: string;
			};
			if (!res.ok || !data.ok) {
				deployError = data.error || 'Deploy failed';
				return;
			}
			deploySuccess = data.release_id
				? `Deployed release ${data.release_id}`
				: 'Deploy succeeded';
			binaryFiles = null;
		} catch {
			deployError = 'Could not reach the panel API';
		} finally {
			deployBusy = false;
		}
	}

	async function deployGit(e: Event) {
		e.preventDefault();
		deployError = '';
		deploySuccess = '';
		const url = gitUrl.trim();
		if (!url) {
			deployError = 'git_url is required';
			return;
		}
		deployBusy = true;
		try {
			const payload: Record<string, string> = { git_url: url };
			const ref = gitRef.trim();
			const cmd = buildCommand.trim();
			const art = artifact.trim();
			if (ref) payload.git_ref = ref;
			if (cmd) payload.build_command = cmd;
			if (art) payload.artifact = art;

			const res = await fetch(`/api/sites/${encodeURIComponent(name)}/deploy/git`, {
				method: 'POST',
				credentials: 'same-origin',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload)
			});
			if (res.status === 401) {
				await goto('/login');
				return;
			}
			const data = (await res.json().catch(() => ({}))) as {
				ok?: boolean;
				error?: string;
				release_id?: string;
			};
			if (!res.ok || !data.ok) {
				deployError = data.error || 'Git deploy failed';
				return;
			}
			deploySuccess = data.release_id
				? `Deployed release ${data.release_id}`
				: 'Deploy succeeded';
		} catch {
			deployError = 'Could not reach the panel API';
		} finally {
			deployBusy = false;
		}
	}

	onMount(load);
</script>

<svelte:head>
	<title>{name} · Webooza</title>
</svelte:head>

<section class="page">
	<p class="crumb"><a href="/sites">Sites</a> <span aria-hidden="true">/</span> <span>{name || '…'}</span></p>

	{#if loading}
		<p class="wbz-muted">Loading…</p>
	{:else if !exists}
		<p class="wbz-error" role="alert">Site not found in config.</p>
	{:else}
		<header class="page-head">
			<div class="title-row">
				<h1>{name}</h1>
				<span class="wbz-badge">{siteType}</span>
			</div>
			<p class="wbz-muted">
				{#if isApp}
					App site · releases under <code>/srv/sites/{name}/</code>
				{:else}
					Document root <code>/srv/sites/{name}/{root}/</code>
				{/if}
			</p>
		</header>

		{#if error}
			<p class="wbz-error" role="alert">{error}</p>
		{/if}

		<section class="wbz-panel" aria-labelledby="overview-heading">
			<h2 id="overview-heading">Overview</h2>
			<p class="lede">
				{#if isApp}
					Deploy a release, then confirm DNS before HTTPS.
				{:else}
					Upload into <code>{root}/</code>, then confirm DNS before HTTPS.
				{/if}
			</p>
			<ul class="facts">
				<li><span>Type</span> <strong>{siteType}</strong></li>
				<li><span>Path</span> <code>/srv/sites/{name}/</code></li>
				<li>
					<span>HTTPS</span>
					<strong>
						{#if !dnsChecked}
							Not checked yet
						{:else if dnsCanCert}
							DNS ready
						{:else}
							DNS not ready
						{/if}
					</strong>
				</li>
			</ul>
		</section>

		{#if isApp}
			<section class="wbz-panel" aria-labelledby="deploy-heading">
				<h2 id="deploy-heading">Deploy</h2>
				<p class="lede">Upload a binary or build from git. Failed health checks roll back automatically.</p>

				<div class="mode" role="group" aria-label="Deploy mode">
					<button
						type="button"
						class={['wbz-btn', 'wbz-btn-ghost', 'mode-btn', deployMode === 'binary' && 'active']}
						disabled={deployBusy}
						onclick={() => (deployMode = 'binary')}
					>
						Binary upload
					</button>
					<button
						type="button"
						class={['wbz-btn', 'wbz-btn-ghost', 'mode-btn', deployMode === 'git' && 'active']}
						disabled={deployBusy}
						onclick={() => (deployMode = 'git')}
					>
						Git deploy
					</button>
				</div>

				{#if deployError}
					<p class="wbz-error" role="alert">{deployError}</p>
				{/if}
				{#if deploySuccess}
					<p class="wbz-ok" role="status">{deploySuccess}</p>
				{/if}

				{#if deployMode === 'binary'}
					<form class="stack" onsubmit={deployBinary}>
						<label class="wbz-field">
							<span>Binary file</span>
							<input
								type="file"
								name="file"
								disabled={deployBusy}
								onchange={(e) => {
									binaryFiles = e.currentTarget.files;
								}}
							/>
						</label>
						<button class="wbz-btn" type="submit" disabled={deployBusy}>
							{deployBusy ? 'Deploying…' : 'Deploy'}
						</button>
					</form>
				{:else}
					<form class="stack" onsubmit={deployGit}>
						<label class="wbz-field">
							<span>Git URL</span>
							<input
								name="git_url"
								bind:value={gitUrl}
								placeholder="https://github.com/org/repo.git"
								required
								disabled={deployBusy}
							/>
						</label>
						<label class="wbz-field">
							<span>Git ref (optional)</span>
							<input name="git_ref" bind:value={gitRef} placeholder="main" disabled={deployBusy} />
						</label>
						<label class="wbz-field">
							<span>Build command (optional)</span>
							<input
								name="build_command"
								bind:value={buildCommand}
								placeholder="go build -o server ."
								disabled={deployBusy}
							/>
						</label>
						<label class="wbz-field">
							<span>Artifact (optional)</span>
							<input
								name="artifact"
								bind:value={artifact}
								placeholder="server"
								disabled={deployBusy}
							/>
						</label>
						<button class="wbz-btn" type="submit" disabled={deployBusy}>
							{deployBusy ? 'Deploying…' : 'Deploy from git'}
						</button>
					</form>
				{/if}
			</section>
		{:else}
			<section class="wbz-panel" aria-labelledby="upload-heading">
				<h2 id="upload-heading">Upload</h2>
				<p class="lede">Files land under <code>{root}/</code> and are served by Caddy.</p>
				{#if uploadOk}
					<p class="wbz-ok" role="status">{uploadOk}</p>
				{/if}
				<form class="stack" onsubmit={upload}>
					<label class="wbz-field">
						<span>Path under {root}/</span>
						<input name="path" bind:value={uploadPath} placeholder="index.html" required />
					</label>
					<label class="wbz-field">
						<span>File</span>
						<input
							type="file"
							onchange={(e) => {
								uploadFile = e.currentTarget.files;
							}}
						/>
					</label>
					<button class="wbz-btn" type="submit" disabled={uploading}>
						{uploading ? 'Uploading…' : 'Upload'}
					</button>
				</form>
			</section>
		{/if}

		<section class="wbz-panel" aria-labelledby="dns-heading">
			<h2 id="dns-heading">DNS checklist</h2>
			<p class="lede">Point the domain at this VPS, then run the check. Certificate issuance waits for a green A record.</p>
			<form
				class="stack dns-form"
				onsubmit={(e) => {
					e.preventDefault();
					checkDNS();
				}}
			>
				<label class="wbz-field">
					<span>Expected A (optional override)</span>
					<input bind:value={expectedA} placeholder="203.0.113.10" />
				</label>
				<button class="wbz-btn" type="submit" disabled={dnsLoading}>
					{dnsLoading ? 'Checking…' : 'Check DNS'}
				</button>
			</form>
			{#if dnsChecked}
				<p class={['summary', dnsCanCert ? 'good' : 'bad']}>
					{dnsCanCert ? 'DNS ready for certificate issuance' : 'DNS not ready for HTTPS yet'}
				</p>
				<table>
					<thead>
						<tr>
							<th>Record</th>
							<th>Status</th>
							<th>Detail</th>
						</tr>
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
		</section>
	{/if}
</section>

<style>
	.page {
		max-width: 44rem;
		display: grid;
		gap: 1.15rem;
	}

	.crumb {
		margin: 0;
		font-size: 0.9rem;
		color: var(--muted);
		font-weight: 700;
	}

	.crumb a {
		text-decoration: none;
		font-weight: 800;
	}

	.crumb span[aria-hidden='true'] {
		margin: 0 0.35rem;
		opacity: 0.5;
	}

	.page-head h1 {
		margin: 0;
		font-size: clamp(1.85rem, 4vw, 2.35rem);
		letter-spacing: -0.03em;
		font-weight: 800;
	}

	.title-row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.65rem;
		margin-bottom: 0.35rem;
	}

	.page-head p {
		margin: 0;
	}

	.facts {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.55rem;
	}

	.facts li {
		display: grid;
		grid-template-columns: 5.5rem 1fr;
		gap: 0.75rem;
		align-items: baseline;
		font-size: 0.95rem;
	}

	.facts span {
		color: var(--muted);
		font-weight: 700;
		font-size: 0.82rem;
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}

	.mode {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin: 0 0 1rem;
	}

	.mode-btn.active {
		background: var(--wbz-ultramarine);
		color: #fff;
		border-color: transparent;
	}

	.stack {
		display: grid;
		gap: 0.9rem;
		max-width: 28rem;
	}

	.stack .wbz-btn {
		justify-self: start;
	}

	.dns-form {
		margin-bottom: 1rem;
	}

	.summary {
		font-weight: 800;
		margin: 0 0 0.75rem;
	}

	.summary.good {
		color: var(--ok);
	}

	.summary.bad {
		color: var(--danger);
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.9rem;
	}

	th,
	td {
		text-align: left;
		padding: 0.5rem 0.35rem;
		border-bottom: 1px solid var(--border);
	}

	th {
		font-size: 0.78rem;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--muted);
		font-weight: 800;
	}

	td.pass {
		color: var(--ok);
		font-weight: 700;
	}

	td.fail {
		color: var(--danger);
		font-weight: 700;
	}

	td.warn,
	td.skip {
		color: var(--warn);
		font-weight: 700;
	}

	code {
		font-size: 0.92em;
	}
</style>
