<script lang="ts">
	import PageHeader from '#lib/components/PageHeader.svelte';
	import LogViewer from '#lib/components/LogViewer.svelte';

	const streams = [{ value: 'backup', label: 'Backup' }];
</script>

<PageHeader title="System logs" subtitle="Host-level backup run logs under /var/log/webor." />

<LogViewer
	listUrl="/api/system/log-files"
	readUrl={(file, lines) => `/api/system/log-files/${encodeURIComponent(file)}?lines=${lines}`}
	{streams}
	downloadPrefix="system"
/>
