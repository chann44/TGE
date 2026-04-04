<script lang="ts">
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
</script>

<div class="soc-page">
	<div class="flex items-center justify-between">
		<h1 class="soc-page-title">Repositories</h1>
		<span class="soc-subtle">{data.repositories.length} found on GitHub</span>
	</div>

	<section class="soc-section">
		<table class="soc-table">
			<thead>
				<tr>
					<th>Repository</th>
					<th>Full Name</th>
					<th>Visibility</th>
					<th>Branch</th>
					<th>Status</th>
					<th>Action</th>
				</tr>
			</thead>
			<tbody>
				{#if data.repositories.length === 0}
					<tr>
						<td colspan="6" class="soc-subtle">No repositories found. Connect GitHub and try again.</td>
					</tr>
				{:else}
					{#each data.repositories as repo}
					<tr class="soc-table-row-link">
						<td>
							<a class="font-medium hover:text-primary" href={`/repos/${repo.id}`}
								>{repo.name}</a
							>
						</td>
						<td class="soc-subtle">{repo.full_name}</td>
						<td class="soc-subtle">{repo.private ? 'Private' : 'Public'}</td>
						<td class="soc-subtle">{repo.default_branch}</td>
						<td>
							<span class={repo.connected ? 'text-emerald-600' : 'soc-subtle'}>
								{repo.connected ? 'Connected' : 'Not connected'}
							</span>
						</td>
						<td>
							{#if repo.connected}
								<button class="soc-btn" type="button" disabled>Connected</button>
							{:else}
								<form method="POST" action="?/connect">
									<input type="hidden" name="repoId" value={repo.id} />
									<button class="soc-btn-primary" type="submit">Connect</button>
								</form>
							{/if}
							<a class="soc-btn ml-2" href={repo.html_url} target="_blank" rel="noreferrer">GitHub</a>
						</td>
					</tr>
					{/each}
				{/if}
			</tbody>
		</table>
	</section>
</div>
