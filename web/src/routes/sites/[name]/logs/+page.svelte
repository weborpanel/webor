<script lang="ts">
	import { onMount } from 'svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import LogViewer from '#lib/components/LogViewer.svelte';
	import { getSite } from '#lib/site/context.svelte.ts';
	import { request } from '#lib/api.ts';

	const site = getSite();

	type Stream = { value: string; label: string };
	type BackendInfo = { id?: string; prefix?: string; enabled?: boolean };

	let apiBackends = $state.raw<BackendInfo[]>([]);
	let ready = $state(false);

	/** Match Go logging.StreamSlug so picker values align with on-disk stream names. */
	function streamSlug(name: string) {
		let s = name.toLowerCase().trim().replace(/_/g, '-').replace(/[^a-z0-9-]+/g, '-').replace(/^-+|-+$/g, '');
		if (!s) s = 'job';
		if (!/^[a-z0-9]/.test(s)) s = `j-${s}`;
		return s.slice(0, 48).replace(/-+$/g, '');
	}

	let streams = $derived.by((): Stream[] => {
		const list: Stream[] = [{ value: 'access', label: 'Access' }];
		if (site.isApp) {
			list.push({ value: 'app', label: site.socketProxy ? 'Proxyd' : 'App' });
			if (site.socketProxy) list.push({ value: 'backend', label: 'Backend' });
		}
		const apis = apiBackends.filter((b) => b.enabled !== false && b.id);
		for (const b of apis) {
			list.push({ value: `api-${streamSlug(b.id!)}`, label: `API · ${b.prefix || b.id}` });
		}
		for (const c of site.cron) list.push({ value: `cron-${streamSlug(c)}`, label: `Cron · ${c}` });
		for (const j of site.jobs) list.push({ value: `job-${streamSlug(j)}`, label: `Job · ${j}` });
		return list;
	});

	async function loadBackends() {
		if (site.isApp) return;
		try {
			const r = await request<{ backends?: BackendInfo[] }>(site.path('/backends'));
			if (r.ok) apiBackends = r.data.backends ?? [];
		} catch {
			apiBackends = [];
		}
	}

	onMount(() => {
		void (async () => {
			await loadBackends();
			ready = true;
		})();
	});
</script>

{#if !ready}
	<p class="wbr-muted">Loading…</p>
{:else if streams.length === 0}
	<EmptyState
		icon="terminal"
		title="No streams to show"
		description="Access logs are always available for static sites. Add an API, cron entry, or job for more streams."
	>
		{#if !site.isApp}
			<a class="wbr-btn" href={site.href('/api')}>Add an API</a>
		{/if}
	</EmptyState>
{:else}
	<LogViewer
		listUrl={site.path('/log-files')}
		readUrl={(file, lines) => site.path(`/log-files/${encodeURIComponent(file)}?lines=${lines}`)}
		{streams}
		downloadPrefix={site.name}
		emptyTitle="No streams to show"
		emptyDescription="Access logs are always available for static sites. Add an API, cron entry, or job for more streams."
	/>
{/if}
