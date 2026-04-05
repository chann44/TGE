<script lang="ts">
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	let tab = $state<'dependency-files' | 'dependencies'>('dependency-files');

	const formattedUpdatedAt = $derived(
		data.repo?.updated_at ? new Date(data.repo.updated_at).toLocaleString() : '-'
	);

	const repositoryRegistries = $derived.by(() => {
		const registries = new Set<string>();
		for (const file of data.dependencyFiles ?? []) {
			if (file.registry) {
				registries.add(file.registry);
			}
		}
		return [...registries].sort((a, b) => a.localeCompare(b));
	});
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
					<div class="flex justify-between"><span class="soc-subtle">Registry</span><span>{repositoryRegistries.length > 0 ? repositoryRegistries.join(', ') : '-'}</span></div>
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

		<div class="flex items-center gap-2">
			<button class="soc-btn" type="button" onclick={() => (tab = 'dependency-files')}
				>Dependency Files</button
			>
			<button class="soc-btn" type="button" onclick={() => (tab = 'dependencies')}>Dependencies</button>
		</div>

		{#if tab === 'dependency-files'}
			<section class="soc-section">
				<div class="soc-section-head"><p class="soc-section-label">Dependency Files</p></div>
				<div class="p-3 text-xs">
					{#if (data.dependencyFiles?.length ?? 0) === 0}
						<p class="soc-subtle">No supported dependency files found.</p>
					{:else}
						<table class="soc-table">
							<thead>
								<tr>
									<th>File</th>
									<th>Path</th>
									<th>Language Manager</th>
									<th>Registry</th>
								</tr>
							</thead>
							<tbody>
								{#each data.dependencyFiles as file}
									<tr>
										<td class="font-medium">{file.file}</td>
										<td class="soc-subtle">{file.path}</td>
										<td>{file.manager}</td>
										<td class="text-primary">{file.registry}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					{/if}
				</div>
			</section>
		{:else}
			<section class="soc-section">
				<div class="soc-section-head"><p class="soc-section-label">Dependencies</p></div>
				<div class="p-3 text-xs">
					{#if (data.dependencies?.length ?? 0) === 0}
						<p class="soc-subtle">No dependencies extracted yet.</p>
					{:else}
						<table class="soc-table">
							<thead>
								<tr>
									<th>Name</th>
									<th>Versions</th>
									<th>Latest</th>
									<th>Creator</th>
									<th>Registry</th>
									<th>Scopes</th>
									<th>Used In</th>
									<th>Last Updated</th>
									<th>Graph</th>
									<th>Link</th>
								</tr>
							</thead>
							<tbody>
								{#each data.dependencies as dep}
									<tr>
										<td class="font-medium">{dep.name}</td>
										<td class="soc-subtle">{dep.version_specs && dep.version_specs.length > 0 ? dep.version_specs.join(', ') : dep.version_spec || '-'}</td>
										<td>{dep.latest_version || '-'}</td>
										<td>{dep.creator || '-'}</td>
										<td class="text-primary">{dep.registry}</td>
										<td>{dep.scopes && dep.scopes.length > 0 ? dep.scopes.join(', ') : dep.scope}</td>
										<td class="soc-subtle">{dep.usage_count > 0 ? `${dep.usage_count} file(s)` : (dep.used_in_files?.length ?? 0) > 0 ? `${dep.used_in_files?.length} file(s)` : '-'}</td>
										<td class="soc-subtle">{dep.last_updated ? new Date(dep.last_updated).toLocaleDateString() : '-'}</td>
										<td>
											{#if dep.dependency_graph && dep.dependency_graph.length > 0}
												<details>
													<summary class="cursor-pointer text-primary">{dep.dependency_graph.length} deps</summary>
													<div class="mt-1 max-h-56 overflow-auto rounded border border-border">
														<table class="soc-table text-[11px]">
															<thead>
																<tr>
																	<th>Name</th>
																	<th>Parent</th>
																	<th>Depth</th>
																	<th>Version</th>
																	<th>Latest</th>
																	<th>Creator</th>
																	<th>Registry</th>
																	<th>Last Updated</th>
																	<th>Link</th>
																</tr>
															</thead>
															<tbody>
																{#each dep.dependency_graph as child}
																	<tr>
																		<td class="font-medium">{child.name}</td>
																		<td class="soc-subtle">{child.parent || '-'}</td>
																		<td>{child.depth}</td>
																		<td class="soc-subtle">{child.version_spec || '-'}</td>
																		<td>{child.latest_version || '-'}</td>
																		<td>{child.creator || '-'}</td>
																		<td class="text-primary">{child.registry || '-'}</td>
																		<td class="soc-subtle">{child.last_updated ? new Date(child.last_updated).toLocaleDateString() : '-'}</td>
																		<td>
																			{#if child.registry_url}
																				<a class="soc-btn" href={child.registry_url} target="_blank" rel="noreferrer">Open</a>
																			{:else}
																				<span class="soc-subtle">-</span>
																			{/if}
																		</td>
																	</tr>
																{/each}
															</tbody>
														</table>
													</div>
												</details>
											{:else}
												<span class="soc-subtle">-</span>
											{/if}
										</td>
										<td>
											{#if dep.registry_url}
												<a class="soc-btn" href={dep.registry_url} target="_blank" rel="noreferrer">Open</a>
											{:else}
												<span class="soc-subtle">-</span>
											{/if}
										</td>
									</tr>
									{#if (dep.scopes && dep.scopes.includes('peer')) || dep.scope === 'peer'}
										<tr>
											<td colspan="10" class="bg-amber-50/70">
												<details>
													<summary class="cursor-pointer py-1 text-[11px] font-medium text-amber-900">
														Peer dependency details for {dep.name}
													</summary>
													<div class="mt-1 space-y-1 rounded border border-amber-200 bg-white p-2 text-[11px]">
														<div class="flex items-center justify-between">
															<span class="soc-subtle">Scopes</span>
															<span>{dep.scopes && dep.scopes.length > 0 ? dep.scopes.join(', ') : dep.scope}</span>
														</div>
														<div class="flex items-center justify-between">
															<span class="soc-subtle">Declared Versions</span>
															<span>{dep.version_specs && dep.version_specs.length > 0 ? dep.version_specs.join(', ') : dep.version_spec || '-'}</span>
														</div>
														<div class="flex items-center justify-between">
															<span class="soc-subtle">Used In</span>
															<span>{dep.used_in_files && dep.used_in_files.length > 0 ? dep.used_in_files.join(', ') : dep.source_file || '-'}</span>
														</div>
													</div>
												</details>
											</td>
										</tr>
									{/if}
								{/each}
							</tbody>
						</table>
					{/if}
				</div>
			</section>
		{/if}
	{/if}
</div>
