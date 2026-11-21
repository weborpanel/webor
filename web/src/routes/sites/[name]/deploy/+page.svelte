<script lang="ts">
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import FileDrop from '#lib/components/FileDrop.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';
	import { overview } from '#lib/stores/overview.svelte.ts';

	const site = getSite();

	type DeployMode = 'binary' | 'git';
	type Result = { ok: boolean; message: string; at: Date };

	let mode = $state<DeployMode>('binary');
	let busy = $state(false);
	let binary = $state<File | null>(null);
	let gitUrl = $state('');
	let gitRef = $state('');
	let buildCommand = $state('');
	let artifact = $state('');
	let history = $state<Result[]>([]);

	function record(ok: boolean, message: string) {
		history = [{ ok, message, at: new Date() }, ...history].slice(0, 5);
		if (ok) toasts.success(message);
		else toasts.error(message);
	}

	async function deployBinary(e: SubmitEvent) {
		e.preventDefault();
		if (!binary) {
			toasts.error('Choose a binary file');
			return;
		}
		busy = true;
		try {
			const body = new FormData();
			body.append('file', binary);
			const r = await request<{ release_id?: string }>(site.path('/deploy'), { method: 'POST', body });
			if (!r.ok) {
				record(false, r.data.error || 'Deploy failed');
				return;
			}
			record(true, r.data.release_id ? `Deployed release ${r.data.release_id}` : 'Deploy succeeded');
			binary = null;
			void overview.refresh();
		} catch (err) {
			record(false, errorMessage(err));
		} finally {
			busy = false;
		}
	}

	async function deployGit(e: SubmitEvent) {
		e.preventDefault();
		const url = gitUrl.trim();
		if (!url) {
			toasts.error('Git URL is required');
			return;
		}
		busy = true;
		try {
			const payload: Record<string, string> = { git_url: url };
			if (gitRef.trim()) payload.git_ref = gitRef.trim();
			if (buildCommand.trim()) payload.build_command = buildCommand.trim();
			if (artifact.trim()) payload.artifact = artifact.trim();
			const r = await request<{ release_id?: string }>(site.path('/deploy/git'), {
				method: 'POST',
				json: payload
			});
			if (!r.ok) {
				record(false, r.data.error || 'Git deploy failed');
				return;
			}
			record(true, r.data.release_id ? `Deployed release ${r.data.release_id}` : 'Deploy succeeded');
			void overview.refresh();
		} catch (err) {
			record(false, errorMessage(err));
		} finally {
			busy = false;
		}
	}
</script>

{#if !site.isApp}
	<EmptyState icon="file" title="Static site" description="Static sites are deployed by uploading files.">
		<a class="wbr-btn" href={site.href('/files')}>Go to Files</a>
	</EmptyState>
{:else}
	<div class="layout">
		<Card title="New release" description="Upload a binary or build from git. A failed health check rolls back automatically.">
			<div class="modes" role="tablist" aria-label="Deploy source">
				<button
					type="button"
					role="tab"
					aria-selected={mode === 'binary'}
					class={['mode', mode === 'binary' && 'on']}
					disabled={busy}
					onclick={() => (mode = 'binary')}
				>
					<Icon name="upload" size={18} />
					<span><strong>Binary upload</strong><small>A prebuilt executable</small></span>
				</button>
				<button
					type="button"
					role="tab"
					aria-selected={mode === 'git'}
					class={['mode', mode === 'git' && 'on']}
					disabled={busy}
					onclick={() => (mode = 'git')}
				>
					<Icon name="git" size={18} />
					<span><strong>Git build</strong><small>Clone, build and release</small></span>
				</button>
			</div>

			{#if mode === 'binary'}
				<form class="wbr-stack" onsubmit={deployBinary}>
					<FileDrop bind:file={binary} label="Choose a binary" disabled={busy} />
					<button class="wbr-btn" type="submit" disabled={busy || !binary}>
						<Icon name="rocket" size={15} />
						{busy ? 'Deploying…' : 'Deploy'}
					</button>
				</form>
			{:else}
				<form class="wbr-stack" onsubmit={deployGit}>
					<label class="wbr-field">
						<span>Repository URL</span>
						<input
							name="git_url"
							bind:value={gitUrl}
							placeholder="https://github.com/org/repo.git"
							required
							disabled={busy}
							spellcheck="false"
						/>
					</label>
					<div class="two">
						<label class="wbr-field">
							<span>Branch, tag or commit</span>
							<input name="git_ref" bind:value={gitRef} placeholder="main" disabled={busy} />
						</label>
						<label class="wbr-field">
							<span>Artifact</span>
							<input name="artifact" bind:value={artifact} placeholder="server" disabled={busy} />
						</label>
					</div>
					<label class="wbr-field">
						<span>Build command</span>
						<input
							name="build_command"
							bind:value={buildCommand}
							placeholder="go build -o server ."
							disabled={busy}
							spellcheck="false"
						/>
						<span class="hint">Leave empty to use the repository's default build.</span>
					</label>
					<button class="wbr-btn" type="submit" disabled={busy}>
						<Icon name="rocket" size={15} />
						{busy ? 'Building & deploying…' : 'Deploy from git'}
					</button>
				</form>
			{/if}
		</Card>

		<Card title="This session" description="Deploys started from this page">
			{#if history.length === 0}
				<EmptyState icon="rocket" title="No deploys yet" description="Results appear here." compact />
			{:else}
				<ul class="history">
					{#each history as h (h.at.getTime())}
						<li class={[h.ok ? 'ok' : 'bad']}>
							<Icon name={h.ok ? 'check' : 'alert'} size={15} />
							<span>{h.message}</span>
							<time>{h.at.toLocaleTimeString()}</time>
						</li>
					{/each}
				</ul>
			{/if}
			{#snippet footer()}
				<a class="wbr-btn wbr-btn-ghost sm" href={site.href('/logs')}><Icon name="terminal" size={14} /> View logs</a>
			{/snippet}
		</Card>
	</div>
{/if}

<style>
	.layout {
		display: grid;
		grid-template-columns: minmax(0, 1.6fr) minmax(16rem, 1fr);
		gap: 1rem;
		align-items: start;
	}

	.modes {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.6rem;
		margin-bottom: 1.1rem;
		max-width: 30rem;
	}

	.mode {
		display: flex;
		gap: 0.6rem;
		align-items: center;
		text-align: left;
		padding: 0.7rem 0.8rem;
		border-radius: var(--radius);
		border: 1px solid var(--border-strong);
		background: var(--surface);
		color: var(--muted);
		font: inherit;
		cursor: pointer;
	}

	.mode span {
		display: grid;
	}

	.mode strong {
		font-size: 0.88rem;
		color: var(--text);
	}

	.mode small {
		font-size: 0.76rem;
	}

	.mode.on {
		border-color: var(--accent);
		background: var(--accent-soft);
		color: var(--accent);
		box-shadow: 0 0 0 1px var(--accent);
	}

	.two {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.75rem;
	}

	.history {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.5rem;
	}

	.history li {
		display: grid;
		grid-template-columns: auto 1fr auto;
		gap: 0.5rem;
		align-items: center;
		font-size: 0.86rem;
	}

	.history .ok {
		color: var(--ok);
	}

	.history .bad {
		color: var(--danger);
	}

	.history span {
		color: var(--text);
		overflow-wrap: anywhere;
	}

	.history time {
		font-size: 0.75rem;
		color: var(--subtle);
	}

	@media (max-width: 1000px) {
		.layout {
			grid-template-columns: 1fr;
		}
	}

	@media (max-width: 520px) {
		.modes,
		.two {
			grid-template-columns: 1fr;
		}
	}
</style>
