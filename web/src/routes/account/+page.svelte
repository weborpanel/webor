<script lang="ts">
	import { onMount } from 'svelte';
	import QRCode from 'qrcode';
	import PageHeader from '#lib/components/PageHeader.svelte';
	import Card from '#lib/components/Card.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import StatusPill from '#lib/components/StatusPill.svelte';
	import CopyField from '#lib/components/CopyField.svelte';
	import type { IconName } from '#lib/icons/paths.ts';
	import { request, errorMessage } from '#lib/api.ts';
	import { toasts } from '#lib/stores/toasts.svelte.ts';
	import { theme, type ThemePref } from '#lib/stores/theme.svelte.ts';
	import { overview } from '#lib/stores/overview.svelte.ts';

	let email = $state('');
	let totpConfirmed = $state(false);
	let loading = $state(true);
	let error = $state('');

	let enrolling = $state(false);
	let secret = $state('');
	let otpauthUrl = $state('');
	let qrDataUrl = $state('');
	let code = $state('');
	let confirming = $state(false);
	let enrollError = $state('');

	type UpdateStatus = {
		current?: string;
		latest?: string;
		channel?: string;
		newer?: boolean;
		same?: boolean;
		base_url?: string;
	};
	let updateChannel = $state('stable');
	let updateCurrent = $state('');
	let updateStatus = $state<UpdateStatus | null>(null);
	let updateError = $state('');
	let updateBusy = $state(false);
	let approvalId = $state('');
	let updateNext = $state<string[]>([]);
	let updateCli = $state('');

	let timezone = $state('');
	let timezoneHost = $state('');
	let timezoneDraft = $state('');
	let timezoneBusy = $state(false);

	const TZ_PRESETS = [
		'UTC',
		'Asia/Kolkata',
		'Asia/Calcutta',
		'America/New_York',
		'America/Los_Angeles',
		'Europe/London',
		'Europe/Berlin',
		'Australia/Sydney'
	] as const;

	const THEMES: { value: ThemePref; label: string; icon: IconName }[] = [
		{ value: 'system', label: 'System', icon: 'monitor' },
		{ value: 'light', label: 'Light', icon: 'sun' },
		{ value: 'dark', label: 'Dark', icon: 'moon' }
	];

	function clearEnroll() {
		secret = '';
		otpauthUrl = '';
		qrDataUrl = '';
		code = '';
		enrollError = '';
	}

	async function load() {
		loading = true;
		try {
			const r = await request<{ email?: string; totp_confirmed?: boolean }>('/api/me');
			if (!r.ok) {
				error = r.data.error || 'Could not load account';
				return;
			}
			email = r.data.email || '';
			totpConfirmed = !!r.data.totp_confirmed;
			error = '';
			const pr = await request<{
				timezone?: string;
				timezone_host?: string;
				timezone_config?: string;
			}>('/api/panel');
			if (pr.ok) {
				timezone = pr.data.timezone || '';
				timezoneHost = pr.data.timezone_host || '';
				timezoneDraft = pr.data.timezone_config || pr.data.timezone || '';
			}
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
		}
	}

	async function saveTimezone(e: SubmitEvent) {
		e.preventDefault();
		timezoneBusy = true;
		try {
			const r = await request<{ timezone?: string; timezone_config?: string; timezone_host?: string }>(
				'/api/panel/timezone',
				{ method: 'PUT', json: { timezone: timezoneDraft.trim() } }
			);
			if (!r.ok) {
				toasts.error(r.data.error || 'Could not save timezone');
				return;
			}
			timezone = r.data.timezone || '';
			timezoneHost = r.data.timezone_host || timezoneHost;
			timezoneDraft = r.data.timezone_config || r.data.timezone || '';
			toasts.success(`Schedules use ${timezone}`);
		} catch (err) {
			toasts.error(errorMessage(err));
		} finally {
			timezoneBusy = false;
		}
	}

	async function beginTotp() {
		enrolling = true;
		enrollError = '';
		qrDataUrl = '';
		try {
			const r = await request<{ secret?: string; otpauth_url?: string }>('/api/auth/totp/begin', { method: 'POST' });
			if (!r.ok) {
				enrollError = r.data.error || 'Could not start enrolment';
				return;
			}
			secret = r.data.secret || '';
			otpauthUrl = r.data.otpauth_url || '';
			if (otpauthUrl) {
				try {
					qrDataUrl = await QRCode.toDataURL(otpauthUrl, {
						errorCorrectionLevel: 'M',
						margin: 2,
						width: 220,
						color: { dark: '#111111', light: '#ffffff' }
					});
				} catch {
					qrDataUrl = '';
				}
			}
		} catch (e) {
			enrollError = errorMessage(e);
		} finally {
			enrolling = false;
		}
	}

	async function loadUpdateStatus() {
		try {
			const r = await request<{ current?: string; channel?: string }>('/api/update/status');
			if (!r.ok) return;
			updateCurrent = r.data.current || '';
			updateChannel = r.data.channel || 'stable';
		} catch {
			/* non-fatal */
		}
	}

	async function checkUpdate() {
		updateBusy = true;
		updateError = '';
		approvalId = '';
		updateNext = [];
		try {
			const r = await request<{
				status?: UpdateStatus;
				cli?: string;
				error?: string;
			}>('/api/update/check', { method: 'POST', json: {} });
			if (!r.ok) {
				updateError = r.data.error || 'Check failed';
				return;
			}
			updateStatus = r.data.status ?? null;
			updateCli = r.data.cli || '';
			if (updateStatus?.channel) updateChannel = updateStatus.channel;
			if (updateStatus?.current) updateCurrent = updateStatus.current;
			if (updateStatus?.newer) {
				toasts.success(`Update available: ${updateStatus.latest}`);
			} else {
				toasts.success('Already up to date');
			}
		} catch (e) {
			updateError = errorMessage(e);
		} finally {
			updateBusy = false;
		}
	}

	async function requestUpdateApproval() {
		updateBusy = true;
		updateError = '';
		try {
			const r = await request<{
				approval_id?: string;
				next?: string[];
				error?: string;
			}>('/api/update/request', { method: 'POST', json: { channel: updateChannel } });
			if (!r.ok) {
				updateError = r.data.error || 'Could not request approval';
				return;
			}
			approvalId = r.data.approval_id || '';
			updateNext = r.data.next ?? [];
			toasts.success('Approval requested — run the CLI steps on the VPS');
		} catch (e) {
			updateError = errorMessage(e);
		} finally {
			updateBusy = false;
		}
	}

	async function confirmTotp(e: SubmitEvent) {
		e.preventDefault();
		const c = code.replace(/\s+/g, '');
		if (!/^\d{6}$/.test(c)) {
			enrollError = 'Enter the 6-digit code from your authenticator app';
			return;
		}
		confirming = true;
		enrollError = '';
		try {
			const r = await request('/api/auth/totp/confirm', { method: 'POST', json: { code: c } });
			if (!r.ok) {
				enrollError = r.data.error || 'Code was not accepted';
				return;
			}
			totpConfirmed = true;
			clearEnroll();
			toasts.success('Two-factor authentication is on');
		} catch (err) {
			enrollError = errorMessage(err);
		} finally {
			confirming = false;
		}
	}

	onMount(() => {
		void load();
		void loadUpdateStatus();
		void overview.ensure(60_000);
	});
</script>

<svelte:head>
	<title>Account · Webor</title>
</svelte:head>

<PageHeader title="Account" subtitle="Sign-in security and panel preferences" />

{#if error}<p class="wbr-error" role="alert">{error}</p>{/if}

<div class="grid">
	<Card title="Profile">
		{#if loading}
			<Skeleton lines={2} />
		{:else}
			<div class="profile">
				<span class="avatar" aria-hidden="true">{(email[0] || '?').toUpperCase()}</span>
				<div>
					<strong>{email || 'Administrator'}</strong>
					<span class="wbr-muted">Panel administrator</span>
				</div>
			</div>
		{/if}
	</Card>

	<Card title="Two-factor authentication" description="Require a code from an authenticator app at sign-in">
		{#snippet actions()}
			{#if !loading}
				<StatusPill tone={totpConfirmed ? 'ok' : 'warn'} label={totpConfirmed ? 'Enabled' : 'Off'} />
			{/if}
		{/snippet}
		{#if loading}
			<Skeleton lines={2} />
		{:else if totpConfirmed}
			<div class="ok-row">
				<Icon name="shield" size={18} />
				<p>Sign-in requires your password and a 6-digit code.</p>
			</div>
		{:else if !secret}
			<p class="wbr-muted small">
				Protect the panel with a time-based one-time code (Google Authenticator, 1Password, Bitwarden, …).
			</p>
			{#if enrollError}<p class="wbr-error">{enrollError}</p>{/if}
			<button type="button" class="wbr-btn" disabled={enrolling} onclick={beginTotp}>
				<Icon name="key" size={15} />
				{enrolling ? 'Starting…' : 'Set up two-factor'}
			</button>
		{:else}
			<ol class="enroll">
				<li>
					<strong>Add the account to your authenticator</strong>
					<span class="wbr-muted">Scan the QR code with Google Authenticator, 1Password, Bitwarden, …</span>
					{#if qrDataUrl}
						<div class="qr-wrap">
							<img class="qr" src={qrDataUrl} alt="Authenticator QR code" width="220" height="220" />
						</div>
					{/if}
					<span class="wbr-muted">Or enter this secret manually.</span>
					<CopyField label="Secret" value={secret} />
					{#if otpauthUrl}
						<a class="otp-link" href={otpauthUrl}><Icon name="link" size={13} /> Open in authenticator app</a>
					{/if}
				</li>
				<li>
					<strong>Enter the current code</strong>
					<form class="confirm" onsubmit={confirmTotp}>
						<label class="wbr-field">
							<span class="visually-hidden">6-digit code</span>
							<input
								bind:value={code}
								inputmode="numeric"
								autocomplete="one-time-code"
								placeholder="123456"
								maxlength="7"
								required
								disabled={confirming}
							/>
						</label>
						<button class="wbr-btn" type="submit" disabled={confirming}>
							{confirming ? 'Verifying…' : 'Verify & enable'}
						</button>
						<button class="wbr-btn wbr-btn-ghost" type="button" disabled={confirming} onclick={clearEnroll}>
							Cancel
						</button>
					</form>
					{#if enrollError}<p class="wbr-error">{enrollError}</p>{/if}
				</li>
			</ol>
		{/if}
	</Card>

	<Card
		title="Schedule timezone"
		description="Cron, backup schedules, and log timestamps in the panel use this zone (not the browser or OS clock). Empty falls back to the server OS timezone."
	>
		{#if loading}
			<Skeleton lines={2} />
		{:else}
			<form class="tz-form" onsubmit={saveTimezone}>
				<label class="wbr-field">
					<span>IANA timezone</span>
					<input
						list="webor-tz-presets"
						bind:value={timezoneDraft}
						placeholder={timezoneHost || 'UTC'}
						disabled={timezoneBusy}
						autocomplete="off"
					/>
					<datalist id="webor-tz-presets">
						{#each TZ_PRESETS as z}
							<option value={z}></option>
						{/each}
					</datalist>
				</label>
				<p class="wbr-muted small">
					Active: <strong>{timezone || 'UTC'}</strong>
					{#if timezoneHost}
						· Host OS: <code>{timezoneHost}</code>
					{/if}
					· Clear the field to follow the host.
				</p>
				<button class="wbr-btn" type="submit" disabled={timezoneBusy}>
					{timezoneBusy ? 'Saving…' : 'Save timezone'}
				</button>
			</form>
		{/if}
	</Card>

	<Card title="Appearance">
		<div class="themes" role="radiogroup" aria-label="Theme">
			{#each THEMES as t (t.value)}
				<button
					type="button"
					role="radio"
					aria-checked={theme.pref === t.value}
					class={['theme', theme.pref === t.value && 'on']}
					onclick={() => theme.set(t.value)}
				>
					<Icon name={t.icon} size={18} />
					{t.label}
				</button>
			{/each}
		</div>
		<p class="wbr-muted small">System follows your operating system setting. Saved in this browser.</p>
	</Card>

	<Card title="Updates" description="Check the release channel and apply from the VPS as root">
		{#snippet actions()}
			<StatusPill tone="neutral" label={updateChannel} />
		{/snippet}
		<p class="wbr-muted small">
			Running
			<strong>Webor {updateStatus?.current || updateCurrent || overview.data?.version || '…'}</strong>
			on channel <code>{updateChannel}</code>.
		</p>
		{#if updateStatus}
			{#if updateStatus.newer}
				<div class="ok-row">
					<Icon name="download" size={18} />
					<p>
						{updateStatus.latest} is available.
						Apply as root on the VPS (panel never writes binaries itself).
					</p>
				</div>
				<CopyField label="Apply update" value={updateCli || `sudo wbr update apply -channel ${updateChannel}`} />
				<div class="update-actions">
					<button type="button" class="wbr-btn" disabled={updateBusy} onclick={requestUpdateApproval}>
						{updateBusy ? 'Working…' : 'Request update approval'}
					</button>
				</div>
				{#if approvalId}
					<p class="wbr-muted small">Approval <code>{approvalId}</code> — run these on the VPS:</p>
					{#each updateNext as cmd (cmd)}
						<CopyField label="CLI" value={cmd} />
					{/each}
				{/if}
			{:else}
				<div class="ok-row">
					<Icon name="check" size={18} />
					<p>Up to date with {updateStatus.latest} on {updateStatus.channel}.</p>
				</div>
			{/if}
		{/if}
		{#if updateError}<p class="wbr-error">{updateError}</p>{/if}
		<div class="update-actions">
			<button type="button" class="wbr-btn wbr-btn-ghost" disabled={updateBusy} onclick={checkUpdate}>
				{updateBusy ? 'Checking…' : 'Check for updates'}
			</button>
		</div>
	</Card>

	<Card title="Keyboard shortcuts">
		<ul class="keys">
			<li><span><kbd>Ctrl</kbd> <kbd>K</kbd></span> Command palette: jump to any site or page</li>
			<li><span><kbd>Esc</kbd></span> Close dialogs and menus</li>
		</ul>
		{#if overview.data?.version}
			<p class="wbr-muted small">Webor {overview.data.version}</p>
		{/if}
	</Card>
</div>

<style>
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(min(24rem, 100%), 1fr));
		gap: 1rem;
		align-items: start;
	}

	.profile {
		display: flex;
		align-items: center;
		gap: 0.85rem;
	}

	.profile div {
		display: grid;
	}

	.profile .wbr-muted {
		font-size: 0.84rem;
	}

	.avatar {
		display: grid;
		place-items: center;
		width: 2.8rem;
		height: 2.8rem;
		border-radius: 50%;
		background: var(--accent);
		color: var(--accent-contrast);
		font-weight: 800;
		font-size: 1.1rem;
	}

	.tz-form {
		display: grid;
		gap: 0.75rem;
		justify-items: start;
	}

	.small {
		font-size: 0.84rem;
		margin: 0 0 0.75rem;
	}

	.ok-row {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		padding: 0.75rem 0.9rem;
		border-radius: var(--radius-sm);
		background: var(--ok-soft);
		color: var(--ok);
	}

	.ok-row p {
		margin: 0;
		color: var(--text);
		font-size: 0.88rem;
	}

	.enroll {
		margin: 0;
		padding-left: 1.2rem;
		display: grid;
		gap: 1.1rem;
	}

	.enroll li {
		display: grid;
		gap: 0.45rem;
	}

	.enroll .wbr-muted {
		font-size: 0.84rem;
	}

	.qr-wrap {
		justify-self: start;
		padding: 0.65rem;
		border-radius: var(--radius-sm);
		background: #fff;
		border: 1px solid var(--border);
	}

	.qr {
		display: block;
		width: 11rem;
		height: 11rem;
		image-rendering: pixelated;
	}

	.otp-link {
		display: inline-flex;
		align-items: center;
		gap: 0.3rem;
		font-size: 0.84rem;
		font-weight: 700;
	}

	.confirm {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		align-items: center;
	}

	.confirm input {
		width: 8rem;
		font-family: var(--mono);
		letter-spacing: 0.15em;
		font-size: 1rem;
	}

	.themes {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 0.5rem;
		margin-bottom: 0.75rem;
	}

	.theme {
		display: grid;
		justify-items: center;
		gap: 0.35rem;
		padding: 0.8rem 0.5rem;
		border: 1px solid var(--border-strong);
		border-radius: var(--radius);
		background: var(--surface);
		color: var(--muted);
		font: inherit;
		font-size: 0.86rem;
		font-weight: 700;
		cursor: pointer;
	}

	.theme:hover {
		background: var(--surface-3);
	}

	.theme.on {
		border-color: var(--accent);
		background: var(--accent-soft);
		color: var(--accent);
		box-shadow: 0 0 0 1px var(--accent);
	}

	.keys {
		list-style: none;
		margin: 0 0 0.75rem;
		padding: 0;
		display: grid;
		gap: 0.55rem;
		font-size: 0.88rem;
	}

	.keys li {
		display: grid;
		grid-template-columns: 6rem 1fr;
		gap: 0.5rem;
		align-items: center;
	}

	.update-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin-top: 0.75rem;
	}
</style>
