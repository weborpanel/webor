<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	let email = $state('');
	let password = $state('');
	let totp = $state('');
	let error = $state('');
	let submitting = $state(false);

	onMount(async () => {
		try {
			const res = await fetch('/api/auth/status', { credentials: 'same-origin' });
			const data = (await res.json().catch(() => ({}))) as {
				ok?: boolean;
				needs_setup?: boolean;
			};
			if (res.ok && data.ok && data.needs_setup) {
				await goto('/setup');
			}
		} catch {
			// Keep login form available if status is unreachable
		}
	});

	async function onSubmit(e: Event) {
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
			await goto('/sites');
		} catch {
			error = 'Could not reach the panel API';
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Log in · Webor</title>
</svelte:head>

<section class="login">
	<h1>Log in</h1>
	<p>Sign in with your admin account.</p>

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
			<span>TOTP code</span>
			<input
				type="text"
				name="totp"
				inputmode="numeric"
				autocomplete="one-time-code"
				bind:value={totp}
				placeholder="000000"
			/>
		</label>
		{#if error}
			<p class="wbr-error" role="alert">{error}</p>
		{/if}
		<button class="wbr-btn" type="submit" disabled={submitting}>
			{submitting ? 'Signing in…' : 'Continue'}
		</button>
	</form>
</section>

<style>
	.login {
		max-width: 26rem;
		padding: 2rem 0 4rem;
	}

	h1 {
		margin: 0 0 0.5rem;
		font-size: clamp(2rem, 5vw, 2.6rem);
		letter-spacing: -0.03em;
		font-weight: 800;
	}

	p {
		margin: 0 0 1.75rem;
		color: var(--muted);
	}

	form {
		display: grid;
		gap: 1rem;
	}

	.wbr-btn {
		justify-self: start;
		margin-top: 0.2rem;
	}
</style>
