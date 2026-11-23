<script lang="ts">
	import { onMount } from 'svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import CopyField from '#lib/components/CopyField.svelte';
	import SecretOnce from '#lib/components/SecretOnce.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';
	import { confirmDialog } from '#lib/stores/confirm.svelte.ts';
	import { overview } from '#lib/stores/overview.svelte.ts';

	const site = getSite();

	type CacheInfo = {
		attached?: boolean;
		engine?: string;
		username?: string;
		prefix?: string;
		host?: string;
		port?: number | string;
		inherited?: boolean;
		parent?: string;
	};
	type ValkeyStatus = {
		installed?: boolean;
		active?: boolean;
		message?: string;
		binary?: string;
	};

	const INSTALL_HINT =
		'# Debian/Ubuntu\nsudo apt-get update && sudo apt-get install -y valkey\nsudo systemctl enable --now valkey-server\n# AlmaLinux / RHEL\nsudo dnf install -y redis\nsudo systemctl enable --now redis';
	const START_HINT = 'sudo systemctl enable --now valkey-server || sudo systemctl enable --now redis';

	let cache = $state.raw<CacheInfo | null>(null);
	let valkey = $state.raw<ValkeyStatus | null>(null);
	let hostInstallHint = $state(INSTALL_HINT);
	let hostStartHint = $state(START_HINT);
	let loading = $state(true);
	let loadError = $state('');
	let busy = $state(false);
	let rotating = $state(false);
	let revealed = $state<{ username: string; password: string; rotated: boolean } | null>(null);
	let installHint = $state('');

	let attached = $derived(!!cache && (cache.attached ?? !!cache.username));
	let address = $derived(cache?.host ? `${cache.host}${cache.port ? `:${cache.port}` : ''}` : '');
	let needsInstall = $derived(!!valkey && !valkey.installed);
	let needsStart = $derived(!!valkey && !!valkey.installed && !valkey.active);
	let blocked = $derived(needsInstall || needsStart);

	async function load() {
		loading = true;
		try {
			const r = await request<
				CacheInfo & {
					cache?: CacheInfo | null;
					valkey?: ValkeyStatus | null;
					install_hint?: string;
					start_hint?: string;
				}
			>(site.path('/cache'));
			if (!r.ok) {
				loadError = r.data.error || 'Could not load cache';
				return;
			}
			loadError = '';
			installHint = '';
			const d = r.data;
			if (d.install_hint) hostInstallHint = d.install_hint;
			if (d.start_hint) hostStartHint = d.start_hint;
			valkey = d.valkey ?? null;
			cache = d.cache ?? {
				attached: d.attached,
				engine: d.engine,
				username: d.username,
				prefix: d.prefix,
				host: d.host,
				port: d.port,
				inherited: d.inherited,
				parent: d.parent
			};
		} catch (e) {
			loadError = errorMessage(e);
		} finally {
			loading = false;
		}
	}

	async function attach(): Promise<void> {
		if (site.isChild || blocked) return;
		busy = true;
		installHint = '';
		try {
			const r = await request<{
				password?: string;
				username?: string;
				cache?: CacheInfo | null;
				install_required?: boolean;
				hint?: string;
			}>(site.path('/cache'), { method: 'POST', json: { engine: 'valkey' } });
			if (r.data.install_required || (!r.ok && r.status === 409)) {
				installHint = r.data.hint || (needsStart ? hostStartHint : hostInstallHint);
				toasts.error(r.data.error || 'Valkey is not ready');
				await load();
				return;
			}
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not attach cache');
				return;
			}
			if (r.data.password) {
				revealed = {
					username: r.data.username || r.data.cache?.username || cache?.username || 'cache',
					password: r.data.password,
					rotated: false
				};
			}
			toasts.success('Cache attached');
			await load();
			void overview.refresh();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			busy = false;
		}
	}

	async function rotate() {
		const ok = await confirmDialog({
			title: 'Rotate cache password?',
			message: 'Apps using the old password stop working until you update their configuration.',
			confirmLabel: 'Rotate password',
			danger: true
		});
		if (!ok) return;
		rotating = true;
		try {
			const r = await request<{ password?: string; username?: string }>(site.path('/cache/rotate-password'), {
				method: 'POST'
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not rotate cache password');
				return;
			}
			if (r.data.password) {
				revealed = { username: r.data.username || cache?.username || 'cache', password: r.data.password, rotated: true };
			}
			toasts.success('Cache password rotated');
			await load();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			rotating = false;
		}
	}

	async function removeCache() {
		const ok = await confirmDialog({
			title: 'Remove cache?',
			message: `All keys under this site's prefix and its Valkey user will be deleted from ${site.name}.`,
			confirmLabel: 'Remove cache',
			danger: true,
			typeToConfirm: site.name
		});
		if (!ok) return;
		busy = true;
		try {
			const r = await request(site.path('/cache'), { method: 'DELETE' });
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not remove cache');
				return;
			}
			revealed = null;
			toasts.success('Cache removed');
			await load();
			void overview.refresh();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			busy = false;
		}
	}

	onMount(() => {
		void load();
	});
</script>

{#if site.isChild}
	<div class="inherit" role="note">
		<Icon name="link" size={16} />
		<p>
			The cache is inherited from
			<a href={`/sites/${encodeURIComponent(site.parent)}/cache`}>{site.parent}</a>. Manage it and rotate
			the password on the parent site.
		</p>
	</div>
{/if}

{#if revealed && !site.isChild}
	<SecretOnce
		subject={revealed.username}
		secret={revealed.password}
		rotated={revealed.rotated}
		ondone={() => (revealed = null)}
	/>
{/if}

<div class="layout">
	<Card title="Valkey cache" description="Its own ACL user and key prefix, so sites can't read each other's keys">
		{#snippet actions()}
			<button type="button" class="wbr-btn wbr-btn-ghost sm" disabled={loading || busy} onclick={load} aria-label="Refresh cache">
				<Icon name="refresh" size={14} />
			</button>
		{/snippet}

		{#if loadError}<p class="wbr-error">{loadError}</p>{/if}

		{#if loading && !cache}
			<Skeleton lines={3} />
		{:else if attached && cache}
			<div class="head">
				<span class="ic" aria-hidden="true"><Icon name="zap" size={18} /></span>
				<div class="title">
					<strong>{cache.username || 'cache'}</strong>
					<span class="wbr-badge neutral">{cache.engine || 'valkey'}</span>
					<span class="wbr-badge ok">attached</span>
				</div>
				{#if !site.isChild}
					<div class="acts">
						<button class="wbr-btn wbr-btn-ghost sm" type="button" disabled={busy || rotating} onclick={rotate}>
							<Icon name="key" size={13} />
							{rotating ? 'Rotating…' : 'Rotate password'}
						</button>
						<button class="wbr-btn-danger sm" type="button" disabled={busy || rotating} onclick={removeCache}>
							<Icon name="trash" size={13} /> Remove
						</button>
					</div>
				{/if}
			</div>
			<div class="fields">
				{#if cache.username}<CopyField label="User" value={cache.username} />{/if}
				{#if cache.prefix}<CopyField label="Key prefix" value={cache.prefix} />{/if}
				{#if !site.isChild && address}<CopyField label="Address" value={address} />{/if}
			</div>
		{:else if blocked && !site.isChild}
			<div class="install" role="status">
				<span class="ic" aria-hidden="true"><Icon name="terminal" size={18} /></span>
				<div class="install-body">
					<strong>{needsInstall ? 'Valkey is not installed' : 'Valkey is not running'}</strong>
					<p>
						{#if needsInstall}
							Log in as root on this VPS and install Valkey. Then click Refresh and Attach cache.
						{:else}
							Log in as root and start the Valkey service. Then click Refresh and Attach cache.
						{/if}
					</p>
					<pre class="cmd">{installHint || (needsInstall ? hostInstallHint : hostStartHint)}</pre>
					<p class="fallback">
						Enable/start <code>valkey-server</code> (not the <code>valkey</code> alias). If port 6379 is
						already taken, use Redis instead — the panel accepts either:
						<code>sudo systemctl enable --now redis-server</code>
					</p>
					<button class="wbr-btn wbr-btn-ghost sm" type="button" disabled={loading || busy} onclick={load}>
						<Icon name="refresh" size={14} /> Check again
					</button>
				</div>
			</div>
		{:else}
			<EmptyState
				icon="zap"
				title="No cache attached"
				description={site.isChild ? `${site.parent} has no cache attached.` : 'Attach a Valkey cache for sessions, queues or page caching.'}
			>
				{#if !site.isChild}
					<button class="wbr-btn" type="button" disabled={busy || blocked} onclick={attach}>
						<Icon name="plus" size={15} />
						{busy ? 'Working…' : 'Attach cache'}
					</button>
				{/if}
			</EmptyState>
		{/if}
	</Card>

	<Card title="Connecting">
		<ul class="tips">
			<li>Authenticate with the user and password (the password is shown once when you attach or rotate).</li>
			<li>Prefix every key with the key prefix; the ACL rejects other keys.</li>
			<li>If Valkey is missing, install it as root first — the panel will not run package installs.</li>
		</ul>
	</Card>
</div>

<style>
	.layout {
		display: grid;
		grid-template-columns: minmax(0, 1.6fr) minmax(16rem, 1fr);
		gap: 1rem;
		align-items: start;
	}

	.inherit {
		display: flex;
		gap: 0.6rem;
		align-items: flex-start;
		padding: 0.8rem 1rem;
		margin-bottom: 1rem;
		border-radius: var(--radius);
		background: var(--info-soft);
		color: var(--info);
	}

	.inherit p {
		margin: 0;
		color: var(--text);
		font-size: 0.9rem;
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

	.fields {
		display: grid;
		gap: 0.45rem;
	}

	.install {
		display: flex;
		gap: 0.75rem;
		align-items: flex-start;
	}

	.install-body {
		display: grid;
		gap: 0.55rem;
		min-width: 0;
		flex: 1;
	}

	.install-body strong {
		font-size: 1rem;
	}

	.install-body p {
		margin: 0;
		color: var(--muted);
		font-size: 0.9rem;
	}

	.cmd {
		margin: 0;
		padding: 0.75rem 0.9rem;
		border-radius: var(--radius-sm);
		border: 1px solid var(--border);
		background: var(--surface-2);
		font-family: var(--mono);
		font-size: 0.82rem;
		line-height: 1.45;
		overflow-x: auto;
		white-space: pre-wrap;
	}

	.fallback {
		font-size: 0.82rem !important;
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
