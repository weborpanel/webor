<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { fetchAuthStatus, HOME_PATH } from '#lib/auth.ts';
	import { NETWORK_ERROR } from '#lib/api.ts';

	let email = $state('');
	let password = $state('');
	let totp = $state('');
	let error = $state('');
	let submitting = $state(false);

	onMount(async () => {
		try {
			const status = await fetchAuthStatus();
			if (status.needs_setup) await goto('/setup');
			else if (status.authenticated) await goto(HOME_PATH);
		} catch {
			// Keep login form available if status is unreachable
		}
	});

	async function onSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		submitting = true;
		try {
			const res = await fetch('/api/auth/login', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				credentials: 'same-origin',
				body: JSON.stringify({ email, password, totp })
			});
			const data = (await res.json().catch(() => ({}))) as { ok?: boolean; error?: string };
			if (!res.ok || !data.ok) {
				error = data.error || 'Login failed';
				return;
			}
			await goto(HOME_PATH);
		} catch {
			error = NETWORK_ERROR;
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Log in · Webor</title>
</svelte:head>

<section class="auth-card">
	<h1>Welcome back</h1>
	<p class="lede">Sign in to manage your sites.</p>

	<form method="post" onsubmit={onSubmit}>
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
		<label class="wbr-field">
			<span>Authenticator code <small class="opt">(if 2FA is enabled)</small></span>
			<input
				type="text"
				name="totp"
				inputmode="numeric"
				autocomplete="one-time-code"
				bind:value={totp}
				placeholder="000000"
				maxlength={8}
			/>
		</label>
		{#if error}
			<p class="wbr-error" role="alert">{error}</p>
		{/if}
		<button class="wbr-btn submit" type="submit" disabled={submitting}>
			{submitting ? 'Signing in…' : 'Sign in'}
		</button>
	</form>
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

	form {
		display: grid;
		gap: 1rem;
	}

	.opt {
		font-weight: 500;
		color: var(--subtle);
	}

	.submit {
		width: 100%;
		margin-top: 0.25rem;
		padding-block: 0.7rem;
	}
</style>
