<script lang="ts">
	import { untrack } from 'svelte';
	import { page } from '$app/state';
	import Tabs from '#lib/components/Tabs.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import StatusPill from '#lib/components/StatusPill.svelte';
	import Skeleton from '#lib/components/Skeleton.svelte';
	import EmptyState from '#lib/components/EmptyState.svelte';
	import { SiteState, setSite } from '#lib/site/context.svelte.ts';
	import { siteSections } from '#lib/siteNav.ts';
	import { siteStatus } from '#lib/siteStatus.ts';
	import { overview, recentSites } from '#lib/stores/overview.svelte.ts';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	const site = new SiteState();
	setSite(site);

	let name = $derived(page.params.name || '');

	$effect(() => {
		const n = name;
		if (!n) return;
		untrack(() => {
			recentSites.visit(n);
			void site.load(n);
		});
	});

	let summary = $derived(overview.sites.find((s) => s.name.toLowerCase() === name.toLowerCase()));
	let status = $derived(summary ? siteStatus(summary) : null);
	let tabs = $derived(siteSections(name, site.type));
	let visitUrl = $derived(`${site.https ? 'https' : 'http'}://${name}`);
</script>

<svelte:head>
	<title>{name} · Webor</title>
</svelte:head>

{#if site.loading && !site.site}
	<div class="head-skeleton">
		<Skeleton width="16rem" height="1.8rem" />
		<Skeleton width="24rem" height="0.9rem" />
	</div>
	<Skeleton height="2.4rem" />
{:else if site.error && !site.site}
	<p class="wbr-error" role="alert">{site.error}</p>
{:else if !site.site}
	<EmptyState icon="globe" title="Site not found" description={`${name} is not in the panel config. It may have been deleted.`}>
		<a class="wbr-btn" href="/sites">Back to sites</a>
	</EmptyState>
{:else}
	<header class="site-head">
		<div class="ident">
			<span class="avatar" aria-hidden="true"><Icon name={site.isApp ? 'rocket' : 'globe'} size={20} /></span>
			<div class="text">
				<div class="title-row">
					<h1>{name}</h1>
					{#if status}<StatusPill tone={status.tone} label={status.label} pulse={status.label === 'Running'} />{/if}
				</div>
				<div class="meta">
					<span class="wbr-badge neutral">{site.type}</span>
					{#if site.https}
						<span class="wbr-badge ok"><Icon name="lock" size={11} /> HTTPS</span>
					{:else}
						<span class="wbr-badge warn">HTTP only</span>
					{/if}
					{#if site.isChild}
						<span class="wbr-badge neutral">
							subdomain of <a href={`/sites/${encodeURIComponent(site.parent)}`}>{site.parent}</a>
						</span>
					{/if}
					<code class="path">/srv/sites/{name}/{site.isApp ? '' : `${site.root}/`}</code>
				</div>
			</div>
		</div>
		<div class="actions">
			<a class="wbr-btn wbr-btn-ghost" href={visitUrl} target="_blank" rel="noopener noreferrer">
				<Icon name="external" size={15} /> Visit
			</a>
			{#if site.isApp}
				<a class="wbr-btn" href={site.href('/deploy')}><Icon name="rocket" size={15} /> Deploy</a>
			{:else}
				<a class="wbr-btn" href={site.href('/files')}><Icon name="upload" size={15} /> Upload</a>
			{/if}
		</div>
	</header>

	<Tabs items={tabs} label={`${name} sections`} />

	{@render children()}
{/if}

<style>
	.head-skeleton {
		display: grid;
		gap: 0.6rem;
		margin-bottom: 1.25rem;
		max-width: 26rem;
	}

	.site-head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1rem;
	}

	.ident {
		display: flex;
		align-items: center;
		gap: 0.9rem;
		min-width: 0;
	}

	.avatar {
		display: grid;
		place-items: center;
		width: 2.9rem;
		height: 2.9rem;
		border-radius: 12px;
		background: var(--accent-soft);
		color: var(--accent);
		flex-shrink: 0;
	}

	.text {
		min-width: 0;
	}

	.title-row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.6rem;
	}

	h1 {
		margin: 0;
		font-size: clamp(1.3rem, 2.6vw, 1.65rem);
		font-weight: 800;
		overflow-wrap: anywhere;
	}

	.meta {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.35rem;
		margin-top: 0.3rem;
	}

	.meta a {
		color: inherit;
	}

	.path {
		font-size: 0.78rem;
		color: var(--muted);
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
</style>
