<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { fetchAuthStatus } from '#lib/auth.ts';
	import { NETWORK_ERROR } from '#lib/api.ts';

	let token = $state('');
	let email = $state('');
	let password = $state('');
	let error = $state('');
	let submitting = $state(false);
	let checking = $state(true);

	onMount(async () => {
		try {
			const status = await fetchAuthStatus();
			if (status.ok && !status.needs_setup) {
				await goto('/login');
				return;
			}
		} catch {
			// Allow setup form if status is unreachable
		} finally {
			checking = false;
		}
	});

	async function onSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		submitting = true;
		try {
			const res = await fetch('/api/auth/setup', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				credentials: 'same-origin',
				body: JSON.stringify({ token, email, password })
			});
			const data = (await res.json().catch(() => ({}))) as { ok?: boolean; error?: string };
			if (!res.ok || !data.ok) {
				error = data.error || 'Setup failed';
				return;
			}
			await goto('/login');
		} catch {
			error = NETWORK_ERROR;
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>First-run setup · Webor</title>
</svelte:head>

<section class="auth-card">
	<span class="step">First-run setup</span>
	<h1>Create the admin account</h1>
	<p class="lede">
		Use the bootstrap token printed by the installer (or in the panel log on first start).
	</p>

	{#if checking}
		<p class="wbr-muted">Checking…</p>
	{:else}
		<form method="post" onsubmit={onSubmit}>
			<label class="wbr-field">
				<span>Bootstrap token</span>
				<input type="text" name="token" autocomplete="off" spellcheck="false" bind:value={token} required />
			</label>
			<label class="wbr-field">
				<span>Email</span>
				<input type="email" name="email" autocomplete="username" bind:value={email} required />
			</label>
			<label class="wbr-field">
				<span>Password</span>
				<input
					type="password"
					name="password"
					autocomplete="new-password"
					bind:value={password}
					required
				/>
			</label>
			{#if error}
				<p class="wbr-error" role="alert">{error}</p>
			{/if}
			<button class="wbr-btn submit" type="submit" disabled={submitting}>
				{submitting ? 'Creating…' : 'Create admin'}
			</button>
		</form>
	{/if}
</section>

<style>
	.auth-card {
		width: min(28rem, 100%);
		margin: auto;
		padding: 2rem;
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		box-shadow: var(--shadow-lg);
	}

	.step {
		font-size: 0.75rem;
		font-weight: 800;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--accent);
	}

	h1 {
		margin: 0.3rem 0 0.35rem;
		font-size: 1.5rem;
		font-weight: 800;
	}

	.lede {
		margin: 0 0 1.5rem;
		color: var(--muted);
		font-size: 0.92rem;
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
</style>
