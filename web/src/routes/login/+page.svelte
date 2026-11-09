<script lang="ts">
	let email = $state('');
	let password = $state('');
	let totp = $state('');
</script>

<svelte:head>
	<title>Log in · Webooza</title>
</svelte:head>

<section class="login">
	<h1>Log in</h1>
	<p>Admin sign-in will land here. Authentication is not wired up yet.</p>

	<form
		method="post"
		onsubmit={(e) => {
			e.preventDefault();
		}}
	>
		<label>
			<span>Email</span>
			<input type="email" name="email" autocomplete="username" bind:value={email} required />
		</label>
		<label>
			<span>Password</span>
			<input
				type="password"
				name="password"
				autocomplete="current-password"
				bind:value={password}
				required
			/>
		</label>
		<label>
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
		<button type="submit">Continue</button>
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

	button:hover {
		filter: brightness(1.05);
	}
</style>
