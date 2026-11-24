<script lang="ts">
	import { onMount } from 'svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import StatusPill from '#lib/components/StatusPill.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';
	import { confirmDialog } from '#lib/stores/confirm.svelte.ts';
	import { overview } from '#lib/stores/overview.svelte.ts';

	const site = getSite();

	type DnsItem = { record: string; host: string; want?: string; got?: string; status: string; detail?: string };

	let dnsItems = $state.raw<DnsItem[]>([]);
	let dnsCanCert = $state(false);
	let dnsChecked = $state(false);
	let dnsLoading = $state(false);
	let dnsError = $state('');
	let expectedA = $state('');

	let httpsBusy = $state(false);
	let httpsError = $state('');

	let subdomains = $state.raw<string[]>([]);
	let subLoading = $state(true);
	let subName = $state('');
	let subBusy = $state(false);
	let subRemoving = $state('');

	type PanelInfo = { domain?: string; parent_site?: string; port?: number };
	let panel = $state.raw<PanelInfo | null>(null);
	let panelLoading = $state(true);
	let panelLabel = $state('panel');
	let panelBusy = $state(false);
	let panelDnsBusy = $state(false);
	let panelDnsChecked = $state(false);
	let panelDnsCanCert = $state(false);
	let panelDnsError = $state('');

	let subPreview = $derived.by(() => {
		const s = subName.trim().toLowerCase();
		if (!s) return '';
		return s.includes('.') ? s : `${s}.${site.name}`;
	});

	let panelPreview = $derived.by(() => {
		const label = panelLabel.trim().toLowerCase();
		if (!label) return '';
		return label.includes('.') ? label : `${label}.${site.name}`;
	});

	let panelOwnedHere = $derived(
		!!panel?.domain && (panel.parent_site || '').toLowerCase() === site.name.toLowerCase()
	);
	let panelScheme = $derived(site.https ? 'https' : 'http');

	function statusTone(s: string): 'ok' | 'warn' | 'danger' | 'neutral' {
		if (s === 'ok' || s === 'pass') return 'ok';
		if (s === 'warn' || s === 'missing') return 'warn';
		if (s === 'fail' || s === 'error' || s === 'mismatch') return 'danger';
		return 'neutral';
	}

	async function checkDNS() {
		dnsLoading = true;
		dnsError = '';
		try {
			const q = expectedA.trim() ? `?a=${encodeURIComponent(expectedA.trim())}` : '';
			const r = await request<{ can_issue_certificate?: boolean; reports?: { items: DnsItem[] }[] }>(
				site.path(`/dns${q}`)
			);
			if (!r.ok) {
				dnsError = r.data.error || 'DNS check failed';
				return;
			}
			dnsItems = r.data.reports?.[0]?.items || [];
			dnsCanCert = !!r.data.can_issue_certificate;
			dnsChecked = true;
		} catch (e) {
			dnsError = errorMessage(e);
		} finally {
			dnsLoading = false;
		}
	}

	async function setHttps(enable: boolean) {
		if (!enable) {
			const ok = await confirmDialog({
				title: 'Disable HTTPS?',
				message: `${site.name} will be served over plain HTTP only.`,
				confirmLabel: 'Disable HTTPS',
				danger: true
			});
			if (!ok) return;
		}
		httpsBusy = true;
		httpsError = '';
		try {
			const r = await request<{ https?: boolean; lab_disabled?: boolean }>(site.path('/https'), {
				method: 'POST',
				json: { enable }
			});
			if (!r.ok) {
				const msg = r.data.error || r.data.message || 'Could not update HTTPS';
				if (r.data.lab_disabled || /\blab\b/i.test(msg)) site.labDisabled = true;
				httpsError = msg;
				return;
			}
			site.setHttps(r.data.https ?? enable);
			toasts.success(enable ? 'HTTPS enabled' : 'HTTPS disabled');
			void overview.refresh();
		} catch (e) {
			httpsError = errorMessage(e);
		} finally {
			httpsBusy = false;
		}
	}

	async function loadSubdomains() {
		if (site.isChild) {
			subLoading = false;
			return;
		}
		subLoading = true;
		try {
			const r = await request<{ subdomains?: string[] }>(site.path('/databases'));
			if (r.ok) subdomains = r.data.subdomains || [];
		} catch {
			// list stays as-is
		} finally {
			subLoading = false;
		}
	}

	async function loadPanel() {
		if (site.isChild) {
			panelLoading = false;
			return;
		}
		panelLoading = true;
		try {
			const r = await request<PanelInfo>('/api/panel');
			if (r.ok) {
				panel = {
					domain: r.data.domain || '',
					parent_site: r.data.parent_site || '',
					port: r.data.port
				};
			}
		} catch {
			// panel card stays as-is
		} finally {
			panelLoading = false;
		}
	}

	async function enablePanel(e: SubmitEvent) {
		e.preventDefault();
		const raw = panelLabel.trim().toLowerCase();
		const label = raw.includes('.') ? raw.split('.')[0]! : raw;
		if (!label) return;
		const elsewhere =
			!!panel?.domain &&
			(panel.parent_site || '').toLowerCase() !== site.name.toLowerCase();
		if (elsewhere) {
			const ok = await confirmDialog({
				title: 'Move panel access?',
				message: `Panel is currently at ${panel?.domain} under ${panel?.parent_site}. Move it to ${panelPreview} on this site?`,
				confirmLabel: 'Move'
			});
			if (!ok) return;
		}
		panelBusy = true;
		try {
			const r = await request<PanelInfo & { label?: string }>('/api/panel/domain', {
				method: 'PUT',
				json: { parent: site.name, label }
			});
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not enable panel access');
				return;
			}
			toasts.success(
				elsewhere
					? `Panel moved to ${r.data.domain || panelPreview}`
					: `Panel published at ${r.data.domain || panelPreview}`
			);
			panelDnsChecked = false;
			panelDnsError = '';
			await loadPanel();
		} catch (err) {
			toasts.error(errorMessage(err));
		} finally {
			panelBusy = false;
		}
	}

	async function disablePanel() {
		const domain = panel?.domain || 'panel domain';
		const ok = await confirmDialog({
			title: 'Disable panel access?',
			message: `${domain} will no longer serve the control panel.`,
			confirmLabel: 'Disable',
			danger: true
		});
		if (!ok) return;
		panelBusy = true;
		try {
			const r = await request('/api/panel/domain', { method: 'DELETE' });
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not disable panel access');
				return;
			}
			toasts.success('Panel access disabled');
			panelDnsChecked = false;
			panelDnsError = '';
			await loadPanel();
		} catch (err) {
			toasts.error(errorMessage(err));
		} finally {
			panelBusy = false;
		}
	}

	async function checkPanelDNS() {
		panelDnsBusy = true;
		panelDnsError = '';
		try {
			const q = expectedA.trim() ? `?a=${encodeURIComponent(expectedA.trim())}` : '';
			const r = await request<{ can_issue_certificate?: boolean }>(`/api/panel/dns${q}`);
			if (!r.ok) {
				panelDnsError = r.data.error || 'DNS check failed';
				return;
			}
			panelDnsCanCert = !!r.data.can_issue_certificate;
			panelDnsChecked = true;
		} catch (e) {
			panelDnsError = errorMessage(e);
		} finally {
			panelDnsBusy = false;
		}
	}

	async function addSubdomain(e: SubmitEvent) {
		e.preventDefault();
		const sub = subPreview;
		if (!sub) return;
		subBusy = true;
		try {
			const r = await request(site.path('/subdomains'), { method: 'POST', json: { name: sub } });
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not add subdomain');
				return;
			}
			toasts.success(`Created ${sub}`);
			subName = '';
			await loadSubdomains();
			void overview.refresh();
		} catch (err) {
			toasts.error(errorMessage(err));
		} finally {
			subBusy = false;
		}
	}

	async function removeSubdomain(sub: string) {
		const ok = await confirmDialog({
			title: `Delete ${sub}?`,
			message: 'This permanently removes:',
			details: ['Its site user', `Files under /srv/sites/${sub}/`, 'Its routes'],
			confirmLabel: 'Delete subdomain',
			danger: true,
			typeToConfirm: sub
		});
		if (!ok) return;
		subRemoving = sub;
		try {
			const r = await request(site.path(`/subdomains/${encodeURIComponent(sub)}`), { method: 'DELETE' });
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not delete subdomain');
				return;
			}
			toasts.success(`Deleted ${sub}`);
			await loadSubdomains();
			void overview.refresh();
		} catch (e) {
			toasts.error(errorMessage(e));
		} finally {
			subRemoving = '';
		}
	}

	onMount(() => {
		void loadSubdomains();
		void loadPanel();
	});
</script>

<div class="layout">
	<div class="col">
		<Card title="HTTPS" description="Certificates are issued and renewed automatically by Caddy">
			<div class="https">
				<span class={['shield', site.https && 'on']} aria-hidden="true">
					<Icon name={site.https ? 'lock' : 'shield'} size={22} />
				</span>
				<div class="https-text">
					<strong>{site.https ? 'HTTPS is on' : 'HTTPS is off'}</strong>
					<span>
						{#if site.https}
							Visitors are redirected to <code>https://{site.name}</code>.
						{:else if site.labDisabled}
							Enabling HTTPS is disabled in this lab environment.
						{:else if dnsChecked && dnsCanCert}
							DNS is ready. You can enable HTTPS now.
						{:else}
							Run the DNS check first; issuance needs a correct A record.
						{/if}
					</span>
				</div>
				{#if site.https}
					<button class="wbr-btn wbr-btn-ghost sm" type="button" disabled={httpsBusy} onclick={() => setHttps(false)}>
						{httpsBusy ? 'Working…' : 'Disable'}
					</button>
				{:else if !site.labDisabled}
					<button
						class="wbr-btn sm"
						type="button"
						disabled={httpsBusy || !dnsCanCert}
						title={dnsCanCert ? undefined : 'Run a passing DNS check first'}
						onclick={() => setHttps(true)}
					>
						{httpsBusy ? 'Enabling…' : 'Enable HTTPS'}
					</button>
				{/if}
			</div>
			{#if httpsError}<p class="wbr-error" role="alert">{httpsError}</p>{/if}
		</Card>

		<Card title="DNS check" description="Point the domain at this server, then check the records">
			<form
				class="dns-form"
				onsubmit={(e) => {
					e.preventDefault();
					void checkDNS();
				}}
			>
				<label class="wbr-field">
					<span>Expected A record</span>
					<input bind:value={expectedA} placeholder="auto-detect (e.g. 203.0.113.10)" spellcheck="false" />
				</label>
				<button class="wbr-btn" type="submit" disabled={dnsLoading}>
					<Icon name="search" size={15} />
					{dnsLoading ? 'Checking…' : 'Check DNS'}
				</button>
			</form>
			{#if dnsError}<p class="wbr-error" role="alert">{dnsError}</p>{/if}
			{#if dnsLoading && !dnsChecked}
				<Skeleton lines={3} />
			{:else if dnsChecked}
				<div class={['summary', dnsCanCert ? 'good' : 'bad']} role="status">
					<Icon name={dnsCanCert ? 'check' : 'alert'} size={16} />
					{dnsCanCert ? 'DNS is ready for certificate issuance' : 'DNS is not ready for HTTPS yet'}
				</div>
				{#if dnsItems.length}
					<div class="table-wrap">
						<table class="wbr-table">
							<thead>
								<tr><th>Record</th><th>Host</th><th>Status</th><th>Value</th></tr>
							</thead>
							<tbody>
								{#each dnsItems as item, i (item.record + item.host + i)}
									<tr>
										<td><code>{item.record}</code></td>
										<td class="host">{item.host}</td>
										<td><StatusPill tone={statusTone(item.status)} label={item.status} /></td>
										<td class="val">
											{item.got || item.detail || '—'}
											{#if item.want && item.got && item.want !== item.got}
												<span class="want">want {item.want}</span>
											{/if}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			{/if}
		</Card>
	</div>

	<div class="col">
		{#if site.isChild}
			<Card title="Subdomain">
				<p class="wbr-muted">
					This site is a subdomain of
					<a href={`/sites/${encodeURIComponent(site.parent)}/domains`}>{site.parent}</a>. Manage subdomains on
					the parent.
				</p>
			</Card>
		{:else}
			<Card title="Subdomains" description="Separate sites that share this site's databases and cache" flush>
				{#if subLoading && subdomains.length === 0}
					<div class="pad"><Skeleton lines={2} /></div>
				{:else if subdomains.length === 0}
					<EmptyState icon="globe" title="No subdomains" description={`e.g. app.${site.name} or api.${site.name}`} compact />
				{:else}
					<ul class="subs">
						{#each subdomains as sub (sub)}
							<li>
								<Icon name="globe" size={15} />
								<a href={`/sites/${encodeURIComponent(sub)}`}>{sub}</a>
								<button
									class="wbr-btn-danger sm"
									type="button"
									disabled={subRemoving === sub}
									onclick={() => removeSubdomain(sub)}
									aria-label={`Delete ${sub}`}
								>
									{#if subRemoving === sub}…{:else}<Icon name="trash" size={13} />{/if}
								</button>
							</li>
						{/each}
					</ul>
				{/if}
				{#snippet footer()}
					<form class="add-sub" onsubmit={addSubdomain}>
						<label class="wbr-field">
							<span class="visually-hidden">New subdomain</span>
							<input
								name="subdomain"
								bind:value={subName}
								placeholder={`app.${site.name}`}
								autocomplete="off"
								spellcheck="false"
								disabled={subBusy}
								required
							/>
							{#if subPreview && subPreview !== subName.trim().toLowerCase()}
								<span class="hint">Creates <code>{subPreview}</code></span>
							{/if}
						</label>
						<button class="wbr-btn" type="submit" disabled={subBusy || !subPreview}>
							<Icon name="plus" size={15} />
							{subBusy ? 'Adding…' : 'Add'}
						</button>
					</form>
				{/snippet}
			</Card>

			<Card title="Panel access" description="Publish the control panel on a subdomain of this site">
				{#if panelLoading && !panel}
					<Skeleton lines={2} />
				{:else if panelOwnedHere && panel}
					<div class="panel-active">
						<a class="panel-host" href={`${panelScheme}://${panel.domain}`} target="_blank" rel="noopener noreferrer">
							{panel.domain}
						</a>
						<p class="wbr-muted panel-dns-note">
							Point an A/AAAA record for this hostname at this server, then check DNS.
						</p>
						<div class="panel-actions">
							<button
								class="wbr-btn wbr-btn-ghost sm"
								type="button"
								disabled={panelDnsBusy}
								onclick={() => void checkPanelDNS()}
							>
								<Icon name="search" size={15} />
								{panelDnsBusy ? 'Checking…' : 'Check DNS'}
							</button>
							<button
								class="wbr-btn wbr-btn-ghost sm"
								type="button"
								disabled={panelBusy}
								onclick={() => void disablePanel()}
							>
								{panelBusy ? 'Working…' : 'Disable'}
							</button>
						</div>
						{#if panelDnsError}<p class="wbr-error" role="alert">{panelDnsError}</p>{/if}
						{#if panelDnsChecked}
							<div class="panel-status" role="status">
								<StatusPill
									tone={panelDnsCanCert ? 'ok' : 'warn'}
									label={panelDnsCanCert ? 'DNS ready' : 'DNS not ready'}
								/>
							</div>
						{/if}
					</div>
				{:else}
					{#if panel?.domain}
						<p class="wbr-muted panel-elsewhere">
							Panel is at <code>{panel.domain}</code> under
							<a href={`/sites/${encodeURIComponent(panel.parent_site || '')}/domains`}>
								{panel.parent_site}
							</a>. Enable below to move it here.
						</p>
					{/if}
					<form class="panel-form" onsubmit={enablePanel}>
						<label class="wbr-field">
							<span>Subdomain label</span>
							<input
								bind:value={panelLabel}
								placeholder="panel"
								autocomplete="off"
								spellcheck="false"
								disabled={panelBusy}
								required
							/>
							{#if panelPreview}
								<span class="hint">Publishes at <code>{panelPreview}</code></span>
							{/if}
						</label>
						<button class="wbr-btn" type="submit" disabled={panelBusy || !panelPreview}>
							{panelBusy ? 'Enabling…' : 'Enable'}
						</button>
					</form>
				{/if}
			</Card>
		{/if}
	</div>
</div>

<style>
	.layout {
		display: grid;
		grid-template-columns: minmax(0, 1.4fr) minmax(17rem, 1fr);
		gap: 1rem;
		align-items: start;
	}

	.col {
		display: grid;
		gap: 1rem;
		min-width: 0;
	}

	.https {
		display: flex;
		align-items: center;
		gap: 0.85rem;
		flex-wrap: wrap;
	}

	.shield {
		display: grid;
		place-items: center;
		width: 2.8rem;
		height: 2.8rem;
		border-radius: 12px;
		background: var(--warn-soft);
		color: var(--warn);
	}

	.shield.on {
		background: var(--ok-soft);
		color: var(--ok);
	}

	.https-text {
		display: grid;
		flex: 1;
		min-width: 12rem;
	}

	.https-text span {
		font-size: 0.85rem;
		color: var(--muted);
	}

	.dns-form {
		display: flex;
		gap: 0.6rem;
		align-items: flex-end;
		flex-wrap: wrap;
		margin-bottom: 0.9rem;
	}

	.dns-form .wbr-field {
		flex: 1;
		min-width: 12rem;
	}

	.summary {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.6rem 0.8rem;
		border-radius: var(--radius-sm);
		font-weight: 700;
		font-size: 0.88rem;
		margin-bottom: 0.75rem;
	}

	.summary.good {
		background: var(--ok-soft);
		color: var(--ok);
	}

	.summary.bad {
		background: var(--warn-soft);
		color: var(--warn);
	}

	.table-wrap {
		overflow-x: auto;
	}

	.host,
	.val {
		font-size: 0.84rem;
		overflow-wrap: anywhere;
	}

	.want {
		display: block;
		color: var(--subtle);
		font-size: 0.76rem;
	}

	.pad {
		padding: 1rem 1.25rem;
	}

	.subs {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.subs li {
		display: grid;
		grid-template-columns: auto 1fr auto;
		gap: 0.6rem;
		align-items: center;
		padding: 0.6rem 1.25rem;
		border-top: 1px solid var(--border);
		color: var(--subtle);
	}

	.subs li:first-child {
		border-top: 0;
	}

	.subs a {
		font-weight: 700;
		text-decoration: none;
		overflow-wrap: anywhere;
	}

	.add-sub {
		display: flex;
		gap: 0.5rem;
		align-items: flex-start;
		width: 100%;
	}

	.add-sub .wbr-field {
		flex: 1;
	}

	.panel-form {
		display: flex;
		gap: 0.5rem;
		align-items: flex-start;
		flex-wrap: wrap;
	}

	.panel-form .wbr-field {
		flex: 1;
		min-width: 10rem;
	}

	.hint {
		display: block;
		margin-top: 0.35rem;
		font-size: 0.8rem;
		color: var(--muted);
	}

	.panel-active {
		display: grid;
		gap: 0.65rem;
	}

	.panel-host {
		font-weight: 700;
		overflow-wrap: anywhere;
	}

	.panel-dns-note {
		margin: 0;
		font-size: 0.85rem;
	}

	.panel-elsewhere {
		margin: 0 0 0.85rem;
		font-size: 0.85rem;
	}

	.panel-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.45rem;
	}

	.panel-status {
		display: flex;
		align-items: center;
	}

	@media (max-width: 1000px) {
		.layout {
			grid-template-columns: 1fr;
		}
	}
</style>
