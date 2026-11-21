<script lang="ts">
	import { onMount } from 'svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import FileDrop from '#lib/components/FileDrop.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';
	import { confirmDialog } from '#lib/stores/confirm.svelte.ts';
	import { overview } from '#lib/stores/overview.svelte.ts';

	const site = getSite();

	type BackendInfo = {
		enabled?: boolean;
		prefix?: string;
		paths?: string[];
		websocket?: boolean;
		exec?: string;
		port?: number;
		listen?: string;
		active?: boolean;
	};

	let backend = $state.raw<BackendInfo | null>(null);
	let loading = $state(true);
	let loadError = $state('');
	let busy = $state(false);
	let saving = $state(false);
	let deploying = $state(false);

	let prefix = $state('/api');
	let extraPaths = $state('');
	let websocket = $state(true);
	let binary = $state<File | null>(null);

	let attached = $derived(!!backend?.enabled);
	let publicBase = $derived.by(() => {
		const p = (prefix.trim() || '/api').replace(/\/$/, '') || '';
		return p === '' || p === '/' ? `https://${site.name}` : `https://${site.name}${p}`;
	});

	function parseExtraPaths(raw: string): string[] {
		return raw
			.split(',')
			.map((p) => p.trim())
			.filter(Boolean);
	}

	function syncForm(b: BackendInfo | null) {
		if (!b?.enabled) {
			prefix = '/api';
			extraPaths = '';
			websocket = true;
			return;
		}
		prefix = b.prefix || '/api';
		const primary = (b.prefix || '/api').replace(/\/$/, '') || '/api';
		extraPaths = (b.paths || [])
			.filter((p) => {
				const n = p.replace(/\/$/, '') || '/';
				return n !== primary;
			})
			.join(', ');
		websocket = b.websocket ?? true;
	}

	async function load() {
		loading = true;
		try {
			const r = await request<{ backend?: BackendInfo | null }>(site.path('/backend'));
			if (!r.ok) {
				loadError = r.data.error || 'Could not load API backend';
				return;
			}
			loadError = '';
			backend = r.data.backend ?? { enabled: false };
			syncForm(backend);
		} catch (e) {
			loadError = errorMessage(e);
		} finally {
			loading = false;
		}
	}

	async function attach() {
		if (site.isApp) return;
		busy = true;
		try {
			const r = await request<{ backend?: BackendInfo }>(site.path('/backend'), {
				method: 'POST',
				json: { websocket: true }
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not attach API backend');
				return;
			}
			toasts.success('API backend attached');
			await load();
			void overview.refresh();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			busy = false;
		}
	}

	async function saveSettings(e: SubmitEvent) {
		e.preventDefault();
		if (!attached) return;
		saving = true;
		try {
			const r = await request<{ backend?: BackendInfo }>(site.path('/backend'), {
				method: 'PATCH',
				json: {
					prefix: prefix.trim() || '/api',
					paths: parseExtraPaths(extraPaths),
					websocket
				}
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not save settings');
				return;
			}
			toasts.success('Settings saved');
			await load();
		} catch (err) {
			toasts.error(errorMessage(err));
		} finally {
			saving = false;
		}
	}

	async function deployBinary(e: SubmitEvent) {
		e.preventDefault();
		if (!binary) {
			toasts.error('Choose a binary file');
			return;
		}
		deploying = true;
		try {
			const body = new FormData();
			body.append('file', binary);
			const r = await request<{ release_id?: string; active?: boolean }>(
				site.path('/backend/deploy'),
				{ method: 'POST', body }
			);
			if (!r.ok) {
				toasts.error(r.data.error || 'Deploy failed');
				return;
			}
			toasts.success(
				r.data.release_id ? `Deployed release ${r.data.release_id}` : 'Binary deployed'
			);
			binary = null;
			await load();
			void overview.refresh();
		} catch (err) {
			toasts.error(errorMessage(err));
		} finally {
			deploying = false;
		}
	}

	async function detach() {
		const ok = await confirmDialog({
			title: 'Detach API backend?',
			message: `The HTTP process and path proxy for ${site.name} will be removed. Static files stay in place.`,
			confirmLabel: 'Detach',
			danger: true,
			typeToConfirm: site.name
		});
		if (!ok) return;
		busy = true;
		try {
			const r = await request(site.path('/backend'), { method: 'DELETE' });
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not detach API backend');
				return;
			}
			toasts.success('API backend detached');
			binary = null;
			await load();
			void overview.refresh();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			busy = false;
		}
	}

	onMount(() => {
		if (!site.isApp) void load();
		else loading = false;
	});
</script>

{#if site.isApp}
	<EmptyState
		icon="server"
		title="API backends attach to static sites"
		description="Use Deploy for app sites."
	>
		<a class="wbr-btn" href={site.href('/deploy')}>Go to Deploy</a>
	</EmptyState>
{:else}
	<div class="layout">
		<Card
			title="HTTP API backend"
			description="Run a process for this site (apex or subdomain). Webor injects PORT and proxies path prefixes."
		>
			{#snippet actions()}
				<button
					type="button"
					class="wbr-btn wbr-btn-ghost sm"
					disabled={loading || busy}
					onclick={load}
					aria-label="Refresh API backend"
				>
					<Icon name="refresh" size={14} />
				</button>
			{/snippet}

			{#if loadError}<p class="wbr-error">{loadError}</p>{/if}

			{#if loading && !backend}
				<Skeleton lines={4} />
			{:else if attached && backend}
				<div class="head">
					<span class="ic" aria-hidden="true"><Icon name="server" size={18} /></span>
					<div class="title">
						<strong>{backend.prefix || '/api'}</strong>
						{#if backend.active}
							<span class="wbr-badge ok">running</span>
						{:else}
							<span class="wbr-badge warn">stopped</span>
						{/if}
						<span class="wbr-badge neutral">attached</span>
					</div>
					<div class="acts">
						<button class="wbr-btn-danger sm" type="button" disabled={busy || saving || deploying} onclick={detach}>
							<Icon name="trash" size={13} /> Detach
						</button>
					</div>
				</div>

				<div class="env-box" role="note">
					<p>
						Bind to <code>os.Getenv("PORT")</code> (and <code>HOST</code>, default
						<code>127.0.0.1</code>). Webor sets <code>PORT</code> / <code>WBR_PORT</code> on the
						service — you never pick a port.
					</p>
					<pre class="snippet">HOST=127.0.0.1 PORT=$PORT ./bin/server</pre>
				</div>

				<form class="settings" onsubmit={saveSettings}>
					<label class="wbr-field">
						<span>Path prefix</span>
						<input
							bind:value={prefix}
							placeholder="/api"
							disabled={saving || busy}
							spellcheck="false"
						/>
						<span class="hint">
							Use <code>/</code> to proxy the whole host (typical for
							<code>api.…</code> subdomains), or <code>/api</code> for path-based APIs.
						</span>
					</label>
					<label class="wbr-field">
						<span>Extra paths (comma-separated)</span>
						<input
							bind:value={extraPaths}
							placeholder="/ws"
							disabled={saving || busy}
							spellcheck="false"
						/>
						<span class="hint">Additional path prefixes proxied to the same process.</span>
					</label>
					<label class="toggle">
						<input type="checkbox" bind:checked={websocket} disabled={saving || busy} />
						<span>
							<strong>WebSocket support</strong>
							<small>Allow WebSocket upgrades through these paths. No Caddy editing needed.</small>
						</span>
					</label>
					<button class="wbr-btn" type="submit" disabled={saving || busy}>
						<Icon name="check" size={15} />
						{saving ? 'Saving…' : 'Save settings'}
					</button>
				</form>

				<form class="deploy" onsubmit={deployBinary}>
					<h3>Deploy binary</h3>
					<p class="deploy-hint">
						Upload an executable that listens on <code>PORT</code>. Public URL:
						<code>{publicBase}/…</code>
					</p>
					<FileDrop bind:file={binary} label="Choose a binary" disabled={deploying || busy} />
					<button class="wbr-btn" type="submit" disabled={deploying || busy || !binary}>
						<Icon name="upload" size={15} />
						{deploying ? 'Deploying…' : 'Deploy'}
					</button>
				</form>
			{:else}
				<EmptyState
					icon="server"
					title="No API backend"
					description="Attach an HTTP process for this site — e.g. api.example.com, or example.com/api."
				>
					<button class="wbr-btn" type="button" disabled={busy} onclick={attach}>
						<Icon name="plus" size={15} />
						{busy ? 'Working…' : 'Attach API backend'}
					</button>
				</EmptyState>
			{/if}
		</Card>

		<Card title="Connecting">
			<ul class="tips">
				<li>
					Read <code>PORT</code> (and optional <code>HOST</code>) from the environment. Webor injects
					them into the systemd unit — do not hard-code a port.
				</li>
				<li>
					Works on apex or subdomain sites (<code>api.example.com</code>,
					<code>backoffice.example.com</code>). Default path <code>/api</code>; use
					<code>/</code> when the whole host is the API.
				</li>
				<li>
					With WebSocket support on, upgrades work on those paths without editing Caddy by hand.
				</li>
				<li>Detach removes the unit and proxy rules; static files are unchanged.</li>
			</ul>
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

	.env-box {
		margin-bottom: 1rem;
		padding: 0.85rem 1rem;
		border-radius: var(--radius);
		background: var(--surface-2, var(--bg-elevated, var(--panel)));
		border: 1px solid var(--border);
	}

	.env-box p {
		margin: 0 0 0.55rem;
		font-size: 0.88rem;
		color: var(--muted);
		line-height: 1.45;
	}

	.snippet {
		margin: 0;
		padding: 0.55rem 0.7rem;
		border-radius: calc(var(--radius) - 2px);
		background: var(--bg, #0f1115);
		color: var(--text);
		font-family: var(--mono);
		font-size: 0.8rem;
		overflow-x: auto;
	}

	.head {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.65rem;
		margin-bottom: 0.9rem;
	}

	.ic {
		display: grid;
		place-items: center;
		width: 2.2rem;
		height: 2.2rem;
		border-radius: 9px;
		background: var(--accent-soft);
		color: var(--accent);
		flex-shrink: 0;
	}

	.title {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.4rem;
		flex: 1;
	}

	.title strong {
		font-family: var(--mono);
	}

	.acts {
		display: flex;
		gap: 0.4rem;
	}

	.settings,
	.deploy {
		display: grid;
		gap: 0.75rem;
		margin-top: 0.25rem;
	}

	.deploy {
		margin-top: 1.25rem;
		padding-top: 1.1rem;
		border-top: 1px solid var(--border);
	}

	.deploy h3 {
		margin: 0;
		font-size: 0.95rem;
	}

	.deploy-hint {
		margin: 0;
		font-size: 0.86rem;
		color: var(--muted);
		line-height: 1.45;
	}

	.toggle {
		display: flex;
		align-items: flex-start;
		gap: 0.55rem;
		font-size: 0.88rem;
		color: var(--text);
	}

	.toggle input {
		margin-top: 0.2rem;
	}

	.toggle span {
		display: grid;
		gap: 0.15rem;
	}

	.toggle strong {
		font-size: 0.88rem;
	}

	.toggle small {
		color: var(--muted);
		font-size: 0.8rem;
		font-weight: 400;
	}

	.tips {
		margin: 0;
		padding-left: 1.1rem;
		display: grid;
		gap: 0.5rem;
		font-size: 0.88rem;
		color: var(--muted);
	}

	@media (max-width: 1000px) {
		.layout {
			grid-template-columns: 1fr;
		}
	}
</style>
