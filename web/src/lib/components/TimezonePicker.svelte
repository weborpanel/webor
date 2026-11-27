<script lang="ts">
	import { listTimeZones } from '#lib/timezones.ts';

	type Props = {
		value?: string;
		disabled?: boolean;
		/** Shown on the empty “follow host” option. */
		hostLabel?: string;
		id?: string;
	};

	let {
		value = $bindable(''),
		disabled = false,
		hostLabel = 'Host OS timezone',
		id = 'timezone-picker'
	}: Props = $props();

	const zones = listTimeZones();
</script>

<select {id} class="tz-select" bind:value {disabled}>
	<option value="">{hostLabel}</option>
	{#each zones as z (z)}
		<option value={z}>{z}</option>
	{/each}
</select>

<style>
	.tz-select {
		width: min(28rem, 100%);
		font-family: var(--mono);
		font-size: 0.88rem;
		font-weight: 500;
	}
</style>
