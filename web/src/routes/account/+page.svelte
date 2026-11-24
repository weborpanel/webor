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
		} catch (e) {
			error = errorMessage(e);
		} finally {
			loading = false;
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
</style>
