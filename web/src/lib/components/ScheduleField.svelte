<script lang="ts">
	import {
		PRESET_OPTIONS,
		WEEKDAY_OPTIONS,
		cronFromPreset,
		describeCron,
		isValidCron,
		matchPreset,
		type SchedulePreset
	} from '#lib/schedule.ts';

	type Props = {
		value?: string;
		/** When true, show an enable/disable toggle (crons). Backups omit this. */
		optional?: boolean;
		disabled?: boolean;
		label?: string;
	};

	let {
		value = $bindable(''),
		optional = false,
		disabled = false,
		label = 'Schedule'
	}: Props = $props();

	let enabled = $state(true);
	let preset = $state<SchedulePreset>('daily');
	let hour = $state(3);
	let minute = $state(0);
	let weekday = $state(0);
	let customExpr = $state('');
	let showAdvanced = $state(false);
	/** Last value we applied or emitted — skips re-syncing our own writes. */
	let appliedValue = $state<string | null>(null);

	function applyFromValue(v: string) {
		const trimmed = v.trim();
		if (optional && !trimmed) {
			enabled = false;
			preset = 'daily';
			hour = 3;
			minute = 0;
			weekday = 0;
			customExpr = '';
			showAdvanced = false;
			return;
		}
		enabled = true;
		const matched = matchPreset(trimmed || '0 3 * * *');
		preset = trimmed ? matched.preset : 'daily';
		hour = matched.hour;
		minute = matched.minute;
		weekday = matched.weekday;
		customExpr = matched.preset === 'custom' ? trimmed : '';
		showAdvanced = matched.preset === 'custom';
	}

	// Sync UI when parent changes bind:value (load / reset), not on our own emits.
	$effect(() => {
		const v = value;
		if (v === appliedValue) return;
		appliedValue = v;
		applyFromValue(v);
	});

	function emitCron(next: string) {
		appliedValue = next;
		if (value !== next) value = next;
	}

	function rebuild() {
		if (optional && !enabled) {
			emitCron('');
			return;
		}
		if (preset === 'custom') {
			emitCron(customExpr.trim());
			return;
		}
		emitCron(cronFromPreset(preset, { hour, minute, weekday }));
	}

	function onToggle(checked: boolean) {
		enabled = checked;
		if (!checked) {
			emitCron('');
			return;
		}
		if (preset === 'custom') {
			if (!customExpr.trim()) customExpr = cronFromPreset('daily', { hour: 3, minute: 0 });
			emitCron(customExpr.trim());
			return;
		}
		emitCron(cronFromPreset(preset, { hour, minute, weekday }));
	}

	function onPresetChange(next: SchedulePreset) {
		preset = next;
		if (next === 'custom') {
			showAdvanced = true;
			if (!customExpr.trim()) {
				customExpr = cronFromPreset('daily', { hour, minute, weekday });
			}
			emitCron(customExpr.trim());
			return;
		}
		showAdvanced = false;
		emitCron(cronFromPreset(next, { hour, minute, weekday }));
	}

	let summary = $derived(describeCron(value));
	let customInvalid = $derived(
		enabled && preset === 'custom' && customExpr.trim() !== '' && !isValidCron(customExpr)
	);
</script>

<div class="schedule" class:disabled>
	<span class="label">{label}</span>

	{#if optional}
		<label class="toggle">
			<input
				type="checkbox"
				checked={enabled}
				{disabled}
				onchange={(e) => onToggle(e.currentTarget.checked)}
			/>
			<span>Enable recurring timer</span>
		</label>
	{/if}

	{#if !optional || enabled}
		<label class="wbr-field">
			<span>Frequency</span>
			<select
				value={preset}
				{disabled}
				onchange={(e) => onPresetChange(e.currentTarget.value as SchedulePreset)}
			>
				{#each PRESET_OPTIONS as opt (opt.value)}
					<option value={opt.value}>{opt.label}</option>
				{/each}
			</select>
		</label>

		{#if preset === 'daily' || preset === 'weekdays' || preset === 'weekly'}
			<div class="row">
				{#if preset === 'weekly'}
					<label class="wbr-field">
						<span>Day</span>
						<select
							value={weekday}
							{disabled}
							onchange={(e) => {
								weekday = Number(e.currentTarget.value);
								rebuild();
							}}
						>
							{#each WEEKDAY_OPTIONS as d (d.value)}
								<option value={d.value}>{d.label}</option>
							{/each}
						</select>
					</label>
				{/if}
				<label class="wbr-field">
					<span>Hour</span>
					<select
						value={hour}
						{disabled}
						onchange={(e) => {
							hour = Number(e.currentTarget.value);
							rebuild();
						}}
					>
						{#each Array.from({ length: 24 }, (_, h) => h) as h (h)}
							<option value={h}>
								{((h % 12) || 12) + (h < 12 ? ' AM' : ' PM')}
							</option>
						{/each}
					</select>
				</label>
				<label class="wbr-field">
					<span>Minute</span>
					<input
						type="number"
						min="0"
						max="59"
						value={minute}
						{disabled}
						onchange={(e) => {
							const n = Number(e.currentTarget.value);
							minute = Number.isFinite(n) ? Math.min(59, Math.max(0, Math.trunc(n))) : 0;
							rebuild();
						}}
					/>
				</label>
			</div>
		{/if}

		{#if preset === 'custom' || showAdvanced}
			<label class="wbr-field">
				<span>Cron expression (5 fields)</span>
				<input
					bind:value={customExpr}
					placeholder="0 3 * * *"
					spellcheck="false"
					{disabled}
					oninput={() => {
						preset = 'custom';
						rebuild();
					}}
				/>
			</label>
			{#if customInvalid}
				<p class="err" role="status">Enter a valid 5-field cron expression.</p>
			{/if}
		{:else}
			<button
				type="button"
				class="advanced"
				{disabled}
				onclick={() => {
					showAdvanced = true;
					preset = 'custom';
					customExpr = value.trim() || cronFromPreset('daily', { hour, minute, weekday });
					rebuild();
				}}
			>
				Advanced (cron expression)
			</button>
		{/if}

		{#if value.trim()}
			<p class="summary" aria-live="polite">{summary}</p>
		{/if}
	{/if}
</div>

<style>
	.schedule {
		display: grid;
		gap: 0.65rem;
	}
	.schedule.disabled {
		opacity: 0.65;
		pointer-events: none;
	}
	.label {
		font-size: 0.82rem;
		font-weight: 600;
		color: var(--text);
	}
	.toggle {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.88rem;
	}
	.row {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(5.5rem, 1fr));
		gap: 0.65rem;
	}
	.summary {
		margin: 0;
		font-size: 0.82rem;
		color: var(--muted);
	}
	.err {
		margin: 0;
		font-size: 0.8rem;
		color: var(--danger, #b91c1c);
	}
	.advanced {
		justify-self: start;
		padding: 0;
		border: 0;
		background: none;
		color: var(--accent, #2563eb);
		font: inherit;
		font-size: 0.8rem;
		cursor: pointer;
		text-decoration: underline;
		text-underline-offset: 2px;
	}
	.advanced:disabled {
		cursor: not-allowed;
		opacity: 0.6;
	}
</style>
