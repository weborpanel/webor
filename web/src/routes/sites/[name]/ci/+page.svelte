<script lang="ts">
	import { onMount } from 'svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import CopyField from '#lib/components/CopyField.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';
	import { confirmDialog } from '#lib/stores/confirm.svelte.ts';

	const site = getSite();

	type TokenRow = {
		id: string;
		name: string;
		site: string;
		scopes: string[];
		created_at: string;
		last_used_at?: string;
		expires_at?: string;
	};

	let tokens = $state.raw<TokenRow[]>([]);
	let loading = $state(true);
	let createBusy = $state(false);
	let tokenName = $state('github-actions');
	let expiresDays = $state(0);
	let plaintext = $state('');
	let panelHint = $state('https://panel.example.com');

	let suggestedKind = $derived.by(() => {
		if (site.isApp) {
			const rt = site.site?.runtime;
			if (rt?.kind === 'node') return 'node';
			return 'binary';
		}
		return 'static';
	});

	let workflowSnippet = $derived(
		[
			'name: deploy',
			'on:',
			'  push:',
			'    branches: [main]',
			'jobs:',
			'  deploy:',
			'    runs-on: ubuntu-latest',
			'    steps:',
			'      - uses: actions/checkout@v4',
			'      # build your artifact here, then:',
			'      - uses: weborpanel/deploy-action@v1',
			'        with:',
			`          panel-url: \${{ vars.WEBOR_PANEL_URL }}`,
			`          token: \${{ secrets.WEBOR_TOKEN }}`,
			`          site: ${site.name}`,
			`          kind: ${suggestedKind}`,
			`          path: ${suggestedKind === 'binary' ? 'bin/server' : suggestedKind === 'node' ? './' : 'dist'}`
		].join('\n')
	);

	async function loadTokens() {
		loading = true;
		try {
			const r = await request<{ tokens?: TokenRow[] }>(site.path('/tokens'));
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not load tokens');
				return;
			}
			tokens = r.data.tokens || [];
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			loading = false;
		}
	}

	async function loadPanelHint() {
		try {
			const r = await request<{ domain?: string; port?: number }>('/api/panel');
			if (r.ok && r.data.domain) {
				panelHint = `https://${r.data.domain}`;
			}
		} catch {
			/* ignore */
		}
	}

	async function createToken(e: SubmitEvent) {
		e.preventDefault();
		createBusy = true;
		plaintext = '';
		try {
			const body: Record<string, unknown> = { name: tokenName.trim() || 'ci' };
			if (expiresDays > 0) body.expires_in_days = expiresDays;
			const r = await request<{ token?: string }>(site.path('/tokens'), {
				method: 'POST',
				json: body
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not create token');
				return;
			}
			plaintext = r.data.token || '';
			toasts.success('Token created — copy it now');
			await loadTokens();
		} catch (err) {
			toasts.error(errorMessage(err));
		} finally {
			createBusy = false;
		}
	}

	async function revoke(id: string, name: string) {
		const ok = await confirmDialog({
			title: 'Revoke deploy token?',
			message: `${name} will stop working in GitHub Actions immediately.`,
			confirmLabel: 'Revoke',
			danger: true
		});
		if (!ok) return;
		try {
			const r = await request(site.path(`/tokens/${encodeURIComponent(id)}`), { method: 'DELETE' });
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not revoke');
				return;
			}
			toasts.success('Token revoked');
			if (plaintext) plaintext = '';
			await loadTokens();
		} catch (e) {
			toasts.error(errorMessage(e));
		}
	}

	function fmtDate(s?: string) {
		if (!s) return '—';
		try {
			return new Date(s).toLocaleString();
		} catch {
			return s;
		}
	}

	onMount(() => {
		void loadTokens();
		void loadPanelHint();
	});
</script>

<section class="ci">
	<p class="lead wbr-muted">
		Mint a site-scoped token for GitHub Actions. CI never creates sites — create this site in the panel first,
		then deploy with <code>weborpanel/deploy-action</code>.
	</p>

	<div class="grid">
		<Card title="Deploy tokens" description="Shown once at create; revoke anytime">
			<form class="create" onsubmit={createToken}>
				<label>
					<span>Name</span>
					<input class="wbr-input" bind:value={tokenName} placeholder="github-actions" disabled={createBusy} />
				</label>
				<label>
					<span>Expires (days, 0 = never)</span>
					<input
						class="wbr-input"
						type="number"
						min="0"
						max="3650"
						bind:value={expiresDays}
						disabled={createBusy}
					/>
				</label>
				<button class="wbr-btn" type="submit" disabled={createBusy}>
					<Icon name="rocket" size={15} />
					{createBusy ? 'Creating…' : 'Create token'}
				</button>
			</form>

			{#if plaintext}
				<div class="once" role="status">
					<p class="warn">Copy this token now — it will not be shown again.</p>
					<CopyField label="WEBOR_TOKEN" value={plaintext} secret />
					<p class="hint wbr-muted">
						Also set repository variable <code>WEBOR_PANEL_URL</code> to your panel URL (e.g.
						<code>{panelHint}</code>).
					</p>
				</div>
			{/if}

			{#if loading}
				<div class="list"><Skeleton height="3rem" /><Skeleton height="3rem" /></div>
			{:else if tokens.length === 0}
				<EmptyState
					icon="rocket"
					title="No deploy tokens"
					description="Create one to paste into GitHub Actions secrets."
				/>
			{:else}
				<ul class="list">
					{#each tokens as t (t.id)}
						<li>
							<div class="meta">
								<strong>{t.name}</strong>
								<span class="wbr-muted scopes">{t.scopes?.join(', ') || '—'}</span>
								<span class="wbr-muted">created {fmtDate(t.created_at)}</span>
								{#if t.last_used_at}
									<span class="wbr-muted">last used {fmtDate(t.last_used_at)}</span>
								{/if}
							</div>
							<button type="button" class="wbr-btn wbr-btn-ghost danger" onclick={() => revoke(t.id, t.name)}>
								Revoke
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</Card>

		<Card title="Workflow snippet" description={`Suggested kind for this site: ${suggestedKind}`}>
			<pre class="snippet">{workflowSnippet}</pre>
			<CopyField label="workflow" value={workflowSnippet} />
			<ul class="notes wbr-muted">
				<li><code>kind: static</code> — upload into <code>public/</code></li>
				<li><code>kind: binary</code> — single Linux executable (+x enforced)</li>
				<li><code>kind: node</code> — zip bundle + optional Node runtime</li>
				<li><code>kind: api</code> — static site HTTP backend (needs <code>backend-id</code>)</li>
				<li>Missing site or kind mismatch fails the job — sites are never auto-created.</li>
			</ul>
		</Card>
	</div>
</section>

<style>
	.ci {
		display: grid;
		gap: 1.25rem;
	}

	.lead {
		margin: 0;
		max-width: 42rem;
		line-height: 1.45;
	}

	.grid {
		display: grid;
		gap: 1.25rem;
		grid-template-columns: repeat(auto-fit, minmax(18rem, 1fr));
		align-items: start;
	}

	.create {
		display: grid;
		gap: 0.75rem;
		margin-bottom: 1rem;
	}

	.create label {
		display: grid;
		gap: 0.35rem;
		font-size: 0.85rem;
	}

	.once {
		display: grid;
		gap: 0.5rem;
		padding: 0.85rem;
		margin-bottom: 1rem;
		border: 1px solid var(--wbr-border, #d8d4cc);
		background: var(--wbr-surface-2, #f7f5f1);
	}

	.warn {
		margin: 0;
		font-weight: 600;
		color: var(--wbr-warn, #9a6700);
	}

	.hint {
		margin: 0;
		font-size: 0.85rem;
	}

	.list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 0.65rem;
	}

	.list li {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		padding: 0.65rem 0;
		border-top: 1px solid var(--wbr-border, #e5e2db);
	}

	.meta {
		display: grid;
		gap: 0.15rem;
		min-width: 0;
	}

	.scopes {
		font-size: 0.8rem;
		font-family: var(--wbr-mono, ui-monospace, monospace);
	}

	.danger {
		color: var(--wbr-danger, #b42318);
	}

	.snippet {
		margin: 0 0 0.75rem;
		padding: 0.85rem;
		overflow: auto;
		font-size: 0.78rem;
		line-height: 1.4;
		background: var(--wbr-surface-2, #f7f5f1);
		border: 1px solid var(--wbr-border, #e5e2db);
		max-height: 18rem;
	}

	.notes {
		margin: 0.85rem 0 0;
		padding-left: 1.1rem;
		display: grid;
		gap: 0.35rem;
		font-size: 0.85rem;
	}
</style>
