<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	let token = $state('');
	let email = $state('');
	let password = $state('');
	let error = $state('');
	let submitting = $state(false);
	let checking = $state(true);

	onMount(async () => {
		try {
			const res = await fetch('/api/auth/status', { credentials: 'same-origin' });
			const data = (await res.json().catch(() => ({}))) as {
				ok?: boolean;
				needs_setup?: boolean;
			};
			if (res.ok && data.ok && data.needs_setup === false) {
				await goto('/login');
				return;
			}
		} catch {
			// Allow setup form if status is unreachable
		} finally {
			checking = false;
		}
	});

	async function onSubmit(e: Event) {
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
			error = 'Could not reach the panel API';
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>First-run setup · Webooza</title>
</svelte:head>

<section class="setup">
	<h1>First-run setup</h1>
	<p>Create the admin account with the bootstrap token from the server.</p>

	{#if checking}
		<p class="muted">Checking…</p>
	{:else}
		<form method="post" onsubmit={onSubmit}>
			<label class="wbz-field">
				<span>Bootstrap token</span>
				<input type="text" name="token" autocomplete="off" bind:value={token} required />
			</label>
			<label class="wbz-field">
				<span>Email</span>
				<input type="email" name="email" autocomplete="username" bind:value={email} required />
			</label>
			<label class="wbz-field">
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
				<p class="wbz-error" role="alert">{error}</p>
			{/if}
			<button class="wbz-btn" type="submit" disabled={submitting}>
				{submitting ? 'Creating…' : 'Create admin'}
			</button>
		</form>
	{/if}
</section>

<style>
	.setup {
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

	.muted {
		margin: 0;
		color: var(--muted);
	}

	form {
		display: grid;
		gap: 1rem;
	}

	.wbz-btn {
		justify-self: start;
		margin-top: 0.2rem;
	}
</style>
