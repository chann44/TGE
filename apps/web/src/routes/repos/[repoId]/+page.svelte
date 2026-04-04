<script lang="ts">
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const formattedUpdatedAt = $derived(
		data.repo?.updated_at ? new Date(data.repo.updated_at).toLocaleString() : '-'
	);
</script>

<div class="soc-page">
	<div class="flex items-center gap-2 text-xs">
		<a class="text-primary" href="/repos">&larr; repos</a>
		<span class="text-muted-foreground">/</span>
		<h1 class="soc-page-title text-base">{data.repo?.name ?? 'Repository'}</h1>
	</div>

	{#if !data.repo}
		<section class="soc-section p-4 text-sm text-muted-foreground">Repository not found.</section>
	{:else}
		<section class="soc-grid-4">
			<div class="soc-section p-3 text-xs">
				<p class="soc-subtle">Status</p>
				<p class="mt-1 text-sm font-semibold {data.repo.connected ? 'text-emerald-600' : ''}">
					{data.repo.connected ? 'Connected' : 'Not connected'}
				</p>
			</div>
			<div class="soc-section p-3 text-xs">
				<p class="soc-subtle">Stars</p>
				<p class="mt-1 text-sm font-semibold">{data.repo.stargazers_count}</p>
			</div>
			<div class="soc-section p-3 text-xs">
				<p class="soc-subtle">Forks</p>
				<p class="mt-1 text-sm font-semibold">{data.repo.forks_count}</p>
			</div>
			<div class="soc-section p-3 text-xs">
				<p class="soc-subtle">Open Issues</p>
				<p class="mt-1 text-sm font-semibold">{data.repo.open_issues_count}</p>
			</div>
		</section>

		<section class="soc-grid-2">
			<div class="soc-section">
				<div class="soc-section-head"><p class="soc-section-label">Repository Info</p></div>
				<div class="space-y-2 p-3 text-xs">
					<div class="flex justify-between"><span class="soc-subtle">Name</span><span>{data.repo.name}</span></div>
					<div class="flex justify-between"><span class="soc-subtle">Full Name</span><span>{data.repo.full_name}</span></div>
					<div class="flex justify-between"><span class="soc-subtle">Visibility</span><span>{data.repo.private ? 'Private' : 'Public'}</span></div>
					<div class="flex justify-between"><span class="soc-subtle">Default Branch</span><span>{data.repo.default_branch}</span></div>
					<div class="flex justify-between"><span class="soc-subtle">Language</span><span>{data.repo.language || '-'}</span></div>
					<div class="flex justify-between"><span class="soc-subtle">Last Update</span><span>{formattedUpdatedAt}</span></div>
				</div>
			</div>

			<div class="soc-section">
				<div class="soc-section-head"><p class="soc-section-label">Actions</p></div>
				<div class="space-y-2 p-3 text-xs">
					<a class="soc-btn-primary inline-block" href={data.repo.html_url} target="_blank" rel="noreferrer">
						Open on GitHub
					</a>
					<p class="soc-subtle">Description</p>
					<p class="text-sm">{data.repo.description || 'No description available.'}</p>
				</div>
			</div>
		</section>
	{/if}
</div>
