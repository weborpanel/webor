<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { fetchAuthStatus, HOME_PATH } from '#lib/auth.ts';
	import { NETWORK_ERROR } from '#lib/api.ts';

	type Step = 'credentials' | 'mfa';

	let step = $state<Step>('credentials');
	let email = $state('');
	let password = $state('');
	let totp = $state('');
	let mfaToken = $state('');
	let error = $state('');
	let submitting = $state(false);
	let totpInput = $state<HTMLInputElement | null>(null);

	onMount(async () => {
		try {
			const status = await fetchAuthStatus();
			if (status.needs_setup) await goto('/setup');
			else if (status.authenticated) await goto(HOME_PATH);
		} catch {
			// Keep login form available if status is unreachable
		}
	});

	async function onCredentials(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		submitting = true;
		try {
			const res = await fetch('/api/auth/login', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				credentials: 'same-origin',
				body: JSON.stringify({ email, password })
			});
			const data = (await res.json().catch(() => ({}))) as {
				ok?: boolean;
				error?: string;
				mfa_required?: boolean;
				mfa_token?: string;
			};
			if (!res.ok || !data.ok) {
				error = data.error || 'Login failed';
				return;
			}
			if (data.mfa_required && data.mfa_token) {
				mfaToken = data.mfa_token;
				totp = '';
				step = 'mfa';
				await tick();
				totpInput?.focus();
				return;
			}
			await goto(HOME_PATH);
		} catch {
			error = NETWORK_ERROR;
		} finally {
			submitting = false;
		}
	}

	async function onMFA(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		const code = totp.replace(/\s+/g, '');
		if (!/^\d{6}$/.test(code)) {
			error = 'Enter the 6-digit code from your authenticator app';
			return;
		}
		submitting = true;
		try {
			const res = await fetch('/api/auth/login/mfa', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				credentials: 'same-origin',
				body: JSON.stringify({ mfa_token: mfaToken, totp: code })
			});
			const data = (await res.json().catch(() => ({}))) as { ok?: boolean; error?: string };
			if (!res.ok || !data.ok) {
				error = data.error || 'Authenticator code was not accepted';
				return;
			}
			await goto(HOME_PATH);
		} catch {
			error = NETWORK_ERROR;
		} finally {
			submitting = false;
		}
	}

	function backToCredentials() {
		step = 'credentials';
		mfaToken = '';
		totp = '';
		error = '';
	}
</script>

<svelte:head>
	<title>Log in · Webor</title>
</svelte:head>

<section class="auth-card">
	{#if step === 'credentials'}
		<h1>Welcome back</h1>
		<p class="lede">Sign in to manage your sites.</p>

		<form method="post" onsubmit={onCredentials}>
			<label class="wbr-field">
				<span>Email</span>
				<input type="email" name="email" autocomplete="username" bind:value={email} required />
			</label>
			<label class="wbr-field">
				<span>Password</span>
				<input
					type="password"
					name="password"
					autocomplete="current-password"
					bind:value={password}
					required
				/>
			</label>
			{#if error}
				<p class="wbr-error" role="alert">{error}</p>
			{/if}
			<button class="wbr-btn submit" type="submit" disabled={submitting}>
				{submitting ? 'Signing in…' : 'Sign in'}
			</button>
		</form>
	{:else}
		<h1>Two-factor authentication</h1>
		<p class="lede">Enter the code from your authenticator app for <strong>{email}</strong>.</p>

		<form method="post" onsubmit={onMFA}>
			<label class="wbr-field">
				<span>Authenticator code</span>
				<input
					bind:this={totpInput}
					type="text"
					name="totp"
					inputmode="numeric"
					autocomplete="one-time-code"
					bind:value={totp}
					placeholder="000000"
					maxlength={8}
					required
				/>
			</label>
			{#if error}
				<p class="wbr-error" role="alert">{error}</p>
			{/if}
			<button class="wbr-btn submit" type="submit" disabled={submitting}>
				{submitting ? 'Verifying…' : 'Verify'}
			</button>
			<button class="wbr-btn wbr-btn-ghost back" type="button" disabled={submitting} onclick={backToCredentials}>
				Back
			</button>
		</form>
	{/if}
</section>

<style>
	.auth-card {
		width: min(26rem, 100%);
		margin: auto;
		padding: 2rem;
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-lg);
	}

	h1 {
		margin: 0 0 0.35rem;
		font-size: 1.6rem;
		font-weight: 800;
	}

	.lede {
		margin: 0 0 1.5rem;
		color: var(--muted);
	}

	.lede strong {
		color: var(--text);
		font-weight: 700;
	}

	form {
		display: grid;
		gap: 1rem;
	}

	.submit {
		width: 100%;
		margin-top: 0.25rem;
		padding-block: 0.7rem;
	}

	.back {
		width: 100%;
	}
</style>
