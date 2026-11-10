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
			<label>
				<span>Bootstrap token</span>
				<input
					type="text"
					name="token"
					autocomplete="off"
					bind:value={token}
					required
				/>
			</label>
			<label>
				<span>Email</span>
				<input type="email" name="email" autocomplete="username" bind:value={email} required />
			</label>
			<label>
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
				<p class="error" role="alert">{error}</p>
			{/if}
			<button type="submit" disabled={submitting}>
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

	.error {
		margin: 0;
		color: #b42318;
		font-weight: 600;
	}

	form {
		display: grid;
		gap: 1rem;
	}

	label {
		display: grid;
		gap: 0.4rem;
		font-weight: 700;
		font-size: 0.92rem;
	}

	input {
		font: inherit;
		padding: 0.75rem 0.9rem;
		border-radius: 12px;
		border: 1px solid color-mix(in srgb, var(--text) 16%, transparent);
		background: color-mix(in srgb, var(--surface) 92%, transparent);
		color: var(--text);
	}

	input:focus {
		outline: 2px solid color-mix(in srgb, var(--accent) 55%, transparent);
		outline-offset: 1px;
	}

	button {
		margin-top: 0.35rem;
		font: inherit;
		font-weight: 800;
		padding: 0.85rem 1.1rem;
		border: 0;
		border-radius: 999px;
		background: var(--wbz-ultramarine);
		color: #fff;
		cursor: pointer;
	}

	button:hover:not(:disabled) {
		filter: brightness(1.05);
	}

	button:disabled {
		opacity: 0.7;
		cursor: wait;
	}
</style>
