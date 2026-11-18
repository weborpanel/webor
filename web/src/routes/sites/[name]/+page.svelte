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

<section class="dash">
	<p class="back"><a href="/sites">← Sites</a></p>
	{#if loading}
		<p class="muted">Loading…</p>
	{:else if !exists}
		<p class="error" role="alert">Site not found in config.</p>
	{:else}
		<header>
			<h1>{name}</h1>
			<p>
				{#if isApp}
					App site · releases under <code>/srv/sites/{name}/</code>
				{:else}
					Document root <code>/srv/sites/{name}/{root}/</code>
				{/if}
			</p>
		</header>

		{#if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		{#if isApp}
			<section class="deploy">
				<h2>Deploy</h2>
				<p class="muted">Upload a binary or build from git. Failed health checks roll back automatically.</p>

				<div class="mode" role="group" aria-label="Deploy mode">
					<button
						type="button"
						class={['mode-btn', deployMode === 'binary' && 'active']}
						disabled={deployBusy}
						onclick={() => (deployMode = 'binary')}
					>
						Binary upload
					</button>
					<button
						type="button"
						class={['mode-btn', deployMode === 'git' && 'active']}
						disabled={deployBusy}
						onclick={() => (deployMode = 'git')}
					>
						Git deploy
					</button>
				</div>

				{#if deployError}
					<p class="error" role="alert">{deployError}</p>
				{/if}
				{#if deploySuccess}
					<p class="ok" role="status">{deploySuccess}</p>
				{/if}

				{#if deployMode === 'binary'}
					<form onsubmit={deployBinary}>
						<label>
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
						<button type="submit" disabled={deployBusy}>
							{deployBusy ? 'Deploying…' : 'Deploy'}
						</button>
					</form>
				{:else}
					<form onsubmit={deployGit}>
						<label>
							<span>Git URL</span>
							<input
								name="git_url"
								bind:value={gitUrl}
								placeholder="https://github.com/org/repo.git"
								required
								disabled={deployBusy}
							/>
						</label>
						<label>
							<span>Git ref (optional)</span>
							<input
								name="git_ref"
								bind:value={gitRef}
								placeholder="main"
								disabled={deployBusy}
							/>
						</label>
						<label>
							<span>Build command (optional)</span>
							<input
								name="build_command"
								bind:value={buildCommand}
								placeholder="go build -o server ."
								disabled={deployBusy}
							/>
						</label>
						<label>
							<span>Artifact (optional)</span>
							<input
								name="artifact"
								bind:value={artifact}
								placeholder="server"
								disabled={deployBusy}
							/>
						</label>
						<button type="submit" disabled={deployBusy}>
							{deployBusy ? 'Deploying…' : 'Deploy from git'}
						</button>
					</form>
				{/if}
			</section>
		{/if}

		<section class="next">
			<h2>Next steps</h2>
			<ol>
				{#if isApp}
					<li>
						<strong>Deploy a release</strong> — use Binary upload or Git deploy above.
					</li>
				{:else}
					<li>
						<strong>Upload files</strong> into <code>{root}/</code> —
						<a href="/sites">open the Sites page</a> and use the upload form for this domain.
					</li>
				{/if}
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
			{#if !isApp}
				<a class="button" href="/sites">Upload files on Sites</a>
			{/if}
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

	.ok {
		color: #067647;
		font-weight: 600;
	}

	.deploy,
	.next {
		margin: 1.5rem 0 0;
	}

	.deploy > .muted {
		margin: 0 0 1rem;
	}

	.mode {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin: 0 0 1rem;
	}

	.mode-btn {
		background: transparent;
		color: var(--text);
		border: 1px solid color-mix(in srgb, var(--text) 20%, transparent);
		padding: 0.45rem 0.9rem;
		font-size: 0.9rem;
		font-weight: 700;
	}

	.mode-btn.active {
		background: var(--wbz-ultramarine);
		color: #fff;
		border-color: transparent;
	}

	form {
		display: grid;
		gap: 0.9rem;
		max-width: 26rem;
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

	input[type='file'] {
		padding: 0.55rem 0;
		border: 0;
		border-radius: 0;
		background: transparent;
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
		justify-self: start;
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
