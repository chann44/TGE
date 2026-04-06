<script lang="ts">
	import DependencyGraph from '$lib/components/repos/DependencyGraph.svelte';
	import type { PageData } from './$types';

	let { data, form }: { data: PageData; form: any } = $props();
	let tab = $state<'dependency-files' | 'dependencies'>('dependency-files');
	let dependenciesView = $state<'table' | 'graph'>('table');
	let selectedGraphNodeId = $state<string>('');
	let expandedTopNodeId = $state<string>('');
	let graphFilterPeer = $state<boolean>(true);
	let graphFilterDev = $state<boolean>(true);
	let graphFilterOptional = $state<boolean>(true);

	type GraphNode = {
		id: string;
		name: string;
		depth: number;
		x: number;
		y: number;
		latestVersion: string;
		versionSpec: string;
		dependencyType: string;
		registry: string;
		manager: string;
		creator: string;
		description: string;
		license: string;
		homepage: string;
		repositoryUrl: string;
		registryUrl: string;
		lastUpdated: string;
		iconUrl: string;
		iconLabel: string;
		isCluster?: boolean;
		clusterCount?: number;
	};

	type GraphEdge = {
		id: string;
		from: string;
		to: string;
		dependencyType: string;
	};

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

	const shouldShowFetchDeps = $derived(
		(data.dependencies?.length ?? 0) === 0 || (data.dependencyFiles?.length ?? 0) === 0
	);

	const graphableDependencies = $derived(data.dependencies ?? []);

	const iconLabelForManager = (manager: string): string => {
		switch ((manager || '').toLowerCase()) {
			case 'npm':
				return 'N';
			case 'pip':
				return 'P';
			case 'go':
				return 'G';
			default:
				return 'D';
		}
	};

	const faviconFromUrl = (rawUrl: string): string => {
		if (!rawUrl) return '';
		try {
			const parsed = new URL(rawUrl);
			return `https://www.google.com/s2/favicons?domain=${parsed.hostname}&sz=32`;
		} catch {
			return '';
		}
	};

	const graphModel = $derived.by(() => {
		const deps = graphableDependencies;
		if (!deps || deps.length === 0) {
			return { nodes: [] as GraphNode[], edges: [] as GraphEdge[], width: 900, height: 260, rootId: '' };
		}

		const maxVisibleDepth = 32;

		const maxRenderedNodes = 1400;
		const horizontalPadding = 80;
		const verticalPadding = 48;

		const shouldIncludeByType = (dependencyType: string): boolean => {
			switch (dependencyType) {
				case 'peer':
					return graphFilterPeer;
				case 'dev':
					return graphFilterDev;
				case 'optional':
					return graphFilterOptional;
				default:
					return true;
			}
		};

		const nodesById = new Map<string, GraphNode>();
		const byDepth = new Map<number, GraphNode[]>();
		const topDepthNameIndex = new Map<string, string>();

		const projectName = data.repo?.name || 'Project';
		const projectID = `project:${projectName}`;
		const projectNode: GraphNode = {
			id: projectID,
			name: projectName,
			depth: 0,
			x: 0,
			y: 0,
			latestVersion: '',
			versionSpec: '',
			dependencyType: 'root',
			registry: '-',
			manager: '-',
			creator: '',
			description: 'Repository root',
			license: '',
			homepage: '',
			repositoryUrl: data.repo?.html_url || '',
			registryUrl: '',
			lastUpdated: '',
			iconUrl: '',
			iconLabel: 'R'
		};
		nodesById.set(projectID, projectNode);
		byDepth.set(0, [projectNode]);

		let maxDepthSeen = 1;

		for (const dep of deps as any[]) {
			if (nodesById.size >= maxRenderedNodes) break;

			const topID = `top:${dep.name}:${dep.latest_version || dep.version_spec || '-'}`;
			if (!nodesById.has(topID)) {
				const topNode: GraphNode = {
					id: topID,
					name: dep.name,
					depth: 1,
					x: 0,
					y: 0,
					latestVersion: dep.latest_version || '-',
					versionSpec: dep.version_spec || '-',
					dependencyType: 'direct',
					registry: dep.registry || '-',
					manager: dep.manager || '-',
					creator: dep.creator || '-',
					description: dep.description || '',
					license: dep.license || '-',
					homepage: dep.homepage || '',
					repositoryUrl: dep.repository_url || '',
					registryUrl: dep.registry_url || '',
					lastUpdated: dep.last_updated || '',
					iconUrl: faviconFromUrl(dep.registry_url || dep.repository_url || ''),
					iconLabel: iconLabelForManager(dep.manager)
				};
				nodesById.set(topID, topNode);
				if (!byDepth.has(1)) byDepth.set(1, []);
				byDepth.get(1)?.push(topNode);
				topDepthNameIndex.set(`${topID}|1|${dep.name}`, topID);
			}

			const filteredChildren = (dep.dependency_graph ?? []).filter((child: any) =>
				shouldIncludeByType(child.dependency_type || 'default')
			);

			for (const child of filteredChildren) {
				if (nodesById.size >= maxRenderedNodes) break;

				const absDepth = Math.max(2, Number(child.depth || 1) + 1);
				if (absDepth > maxDepthSeen) maxDepthSeen = absDepth;

				const depType = child.dependency_type || 'default';
				const childID = `${topID}:${absDepth}:${depType}:${child.parent || dep.name}:${child.name}:${child.latest_version || child.version_spec || '-'}`;
				if (!nodesById.has(childID)) {
					const node: GraphNode = {
						id: childID,
						name: child.name,
						depth: absDepth,
						x: 0,
						y: 0,
						latestVersion: child.latest_version || '-',
						versionSpec: child.version_spec || '-',
						dependencyType: depType,
						registry: child.registry || '-',
						manager: child.manager || '-',
						creator: child.creator || '-',
						description: child.description || '',
						license: child.license || '-',
						homepage: child.homepage || '',
						repositoryUrl: child.repository_url || '',
						registryUrl: child.registry_url || '',
						lastUpdated: child.last_updated || '',
						iconUrl: faviconFromUrl(child.registry_url || child.repository_url || ''),
						iconLabel: iconLabelForManager(child.manager)
					};
					nodesById.set(childID, node);
					if (!byDepth.has(absDepth)) byDepth.set(absDepth, []);
					byDepth.get(absDepth)?.push(node);
					if (!topDepthNameIndex.has(`${topID}|${absDepth}|${child.name}`)) {
						topDepthNameIndex.set(`${topID}|${absDepth}|${child.name}`, childID);
					}
				}
			}
		}

		const laneGapX = 190;
		const laneGapY = 92;
		const bandGapY = 130;
		const maxColsDepth1 = 7;
		const maxColsOther = 9;

		const sortedDepths = [...byDepth.keys()].sort((a, b) => a - b);
		let yCursor = verticalPadding + 16;
		let widestRow = 1;

		for (const depth of sortedDepths) {
			const nodes = byDepth.get(depth) ?? [];
			nodes.sort((a, b) => {
				if (a.dependencyType !== b.dependencyType) {
					return a.dependencyType.localeCompare(b.dependencyType);
				}
				return a.name.localeCompare(b.name);
			});

			if (depth === 0) {
				nodes[0].x = 0;
				nodes[0].y = yCursor;
				yCursor += bandGapY;
				continue;
			}

			const maxCols = depth === 1 ? maxColsDepth1 : maxColsOther;
			const rows = Math.max(1, Math.ceil(nodes.length / maxCols));
			for (let row = 0; row < rows; row++) {
				const from = row * maxCols;
				const to = Math.min(nodes.length, from + maxCols);
				const rowNodes = nodes.slice(from, to);
				widestRow = Math.max(widestRow, rowNodes.length);
				const rowWidth = (rowNodes.length - 1) * laneGapX;
				const rowStartX = -rowWidth / 2;
				rowNodes.forEach((node, col) => {
					node.x = rowStartX + col * laneGapX;
					node.y = yCursor + row * laneGapY;
				});
			}

			yCursor += rows * laneGapY + bandGapY;
		}

		const width = Math.max(1300, horizontalPadding * 2 + Math.max(1, widestRow - 1) * laneGapX);
		const height = Math.max(700, yCursor + verticalPadding);

		for (const nodes of byDepth.values()) {
			for (const node of nodes) {
				node.x += width / 2;
			}
		}

		const edges: GraphEdge[] = [];
		for (const dep of deps as any[]) {
			const topID = `top:${dep.name}:${dep.latest_version || dep.version_spec || '-'}`;
			if (!nodesById.has(topID)) continue;

			edges.push({
				id: `${projectID}->${topID}`,
				from: projectID,
				to: topID,
				dependencyType: 'direct'
			});

			const filteredChildren = (dep.dependency_graph ?? []).filter((child: any) =>
				shouldIncludeByType(child.dependency_type || 'default')
			);
			for (const child of filteredChildren) {
				const depth = Math.max(2, Number(child.depth || 1) + 1);
				const depType = child.dependency_type || 'default';
				const childId = `${topID}:${depth}:${depType}:${child.parent || dep.name}:${child.name}:${child.latest_version || child.version_spec || '-'}`;
				if (!nodesById.has(childId)) continue;

				let from = topID;
				if (depth > 2) {
					const indexedParent = topDepthNameIndex.get(`${topID}|${depth - 1}|${child.parent}`);
					if (indexedParent) from = indexedParent;
				}
				edges.push({
					id: `${from}->${childId}`,
					from,
					to: childId,
					dependencyType: child.dependency_type || 'default'
				});
			}
		}

		const fullNodes = [...nodesById.values()];
		const visibleNodes = fullNodes.filter((node) => node.depth <= maxVisibleDepth);
		const visibleNodeIDs = new Set(visibleNodes.map((node) => node.id));

		const outgoingByFrom = new Map<string, GraphEdge[]>();
		for (const edge of edges) {
			if (!outgoingByFrom.has(edge.from)) {
				outgoingByFrom.set(edge.from, []);
			}
			outgoingByFrom.get(edge.from)?.push(edge);
		}

		const filteredEdges = edges.filter((edge) => visibleNodeIDs.has(edge.from) && visibleNodeIDs.has(edge.to));

		return {
			nodes: visibleNodes,
			edges: filteredEdges,
			width,
			height,
			rootId: projectID
		};
	});

	const displayedGraphModel = $derived.by(() => {
		const base = graphModel;
		if (!expandedTopNodeId) {
			const nodes = base.nodes.filter((node) => node.depth <= 1);
			const ids = new Set(nodes.map((n) => n.id));
			const edges = base.edges.filter((edge) => ids.has(edge.from) && ids.has(edge.to));
			return { ...base, nodes, edges };
		}

		const edgeMap = new Map<string, GraphEdge[]>();
		for (const edge of base.edges) {
			if (!edgeMap.has(edge.from)) edgeMap.set(edge.from, []);
			edgeMap.get(edge.from)?.push(edge);
		}

		const keepNodeIDs = new Set<string>();
		for (const node of base.nodes) {
			if (node.depth <= 1) keepNodeIDs.add(node.id);
		}
		const queue = [expandedTopNodeId];
		while (queue.length > 0) {
			const current = queue.shift() as string;
			if (keepNodeIDs.has(current)) {
				for (const edge of edgeMap.get(current) ?? []) {
					if (!keepNodeIDs.has(edge.to)) queue.push(edge.to);
				}
				continue;
			}
			keepNodeIDs.add(current);
			for (const edge of edgeMap.get(current) ?? []) {
				if (!keepNodeIDs.has(edge.to)) queue.push(edge.to);
			}
		}

		const nodes = base.nodes.filter((node) => keepNodeIDs.has(node.id));
		const edges = base.edges.filter((edge) => keepNodeIDs.has(edge.from) && keepNodeIDs.has(edge.to));
		return { ...base, nodes, edges };
	});

	const selectedGraphNode = $derived.by(() => {
		const { nodes } = displayedGraphModel;
		if (nodes.length === 0 || !selectedGraphNodeId) return null;
		return nodes.find((n) => n.id === selectedGraphNodeId) ?? null;
	});

	$effect(() => {
		const { nodes } = displayedGraphModel;
		if (nodes.length === 0) {
			selectedGraphNodeId = '';
			expandedTopNodeId = '';
			return;
		}
		if (selectedGraphNodeId && !nodes.some((node) => node.id === selectedGraphNodeId)) {
			selectedGraphNodeId = '';
		}
		if (expandedTopNodeId && !nodes.some((node) => node.id === expandedTopNodeId)) {
			expandedTopNodeId = '';
		}
	});

	function resetGraphViewport() {
		selectedGraphNodeId = '';
		expandedTopNodeId = '';
	}

	const pixiGraphNodes = $derived.by(() =>
		displayedGraphModel.nodes.map((node) => ({
			id: node.id,
			label: node.name,
			type: node.dependencyType
		}))
	);

	const pixiGraphEdges = $derived.by(() =>
		displayedGraphModel.edges.map((edge) => ({
			source: edge.from,
			target: edge.to,
			type: edge.dependencyType
		}))
	);

	function onGraphNodeClick(event: CustomEvent<{ nodeId: string }>) {
		const nodeId = event.detail.nodeId;
		selectedGraphNodeId = nodeId;
		if (nodeId.startsWith('top:')) {
			expandedTopNodeId = expandedTopNodeId === nodeId ? '' : nodeId;
		} else if (nodeId.startsWith('project:')) {
			expandedTopNodeId = '';
		}
	}
</script>

<div class="soc-page">
	{#if form && typeof form === 'object' && form.action === 'fetchDeps' && 'queued' in form && form.queued}
		<section class="soc-section mb-3 border border-emerald-300 bg-emerald-50 p-3 text-xs text-emerald-800">
			Dependency fetch job queued (status: {form.syncStatus ?? 'queued'}). Refresh in a moment to see updated results.
		</section>
	{/if}

	{#if form && typeof form === 'object' && form.action === 'runScan' && form.success !== false}
		<section class="soc-section mb-3 border border-emerald-300 bg-emerald-50 p-3 text-xs text-emerald-800">
			{form.message ?? `Scan run queued (status: ${form.scanStatus ?? 'queued'}).`}
		</section>
	{/if}

	{#if form && typeof form === 'object' && 'message' in form && form.message}
		<section class="soc-section mb-3 border border-rose-300 bg-rose-50 p-3 text-xs text-rose-800">
			{form.message}
		</section>
	{/if}

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
					<form method="POST" action="?/fetchDeps" class="inline-block">
						<button class={shouldShowFetchDeps ? 'soc-btn-primary' : 'soc-btn'} type="submit">
							Fetch Deps
						</button>
					</form>
					<form method="POST" action="?/runScan" class="inline-block">
						<button class="soc-btn-primary" type="submit">Run Scan</button>
					</form>
					{#if data.syncStatus}
						<div class="rounded border border-border p-2 text-[11px]">
							<div class="flex items-center justify-between">
								<span class="soc-subtle">Sync status</span>
								<span>{data.syncStatus}</span>
							</div>
							{#if data.lastSyncedAt}
								<div class="flex items-center justify-between">
									<span class="soc-subtle">Last synced</span>
									<span>{new Date(data.lastSyncedAt).toLocaleString()}</span>
								</div>
							{/if}
							{#if data.syncError}
								<p class="mt-1 text-rose-700">{data.syncError}</p>
							{/if}
						</div>
					{/if}
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
					<div class="mb-3 flex items-center justify-between gap-2">
						<div class="flex items-center gap-2">
							<button class="soc-btn" type="button" onclick={() => (dependenciesView = 'table')}>Table View</button>
							<button class="soc-btn" type="button" onclick={() => (dependenciesView = 'graph')}>Graph View</button>
						</div>
						{#if dependenciesView === 'graph'}
							<div class="flex flex-wrap items-center gap-2">
								<button class="soc-btn" type="button" onclick={resetGraphViewport}>Reset View</button>
								<button
									type="button"
									class={graphFilterPeer ? 'soc-btn-primary' : 'soc-btn'}
									onclick={() => (graphFilterPeer = !graphFilterPeer)}
								>
									peer
								</button>
								<button
									type="button"
									class={graphFilterDev ? 'soc-btn-primary' : 'soc-btn'}
									onclick={() => (graphFilterDev = !graphFilterDev)}
								>
									dev
								</button>
								<button
									type="button"
									class={graphFilterOptional ? 'soc-btn-primary' : 'soc-btn'}
									onclick={() => (graphFilterOptional = !graphFilterOptional)}
								>
									optional
								</button>
								<span class="rounded border border-border bg-muted px-2 py-1 text-[11px]">
									{displayedGraphModel.nodes.length} nodes / {displayedGraphModel.edges.length} edges
								</span>
								{#if expandedTopNodeId}
									<span class="rounded border border-border bg-muted px-2 py-1 text-[11px]">Expanded branch</span>
								{:else}
									<span class="rounded border border-border bg-muted px-2 py-1 text-[11px]">Click top-level dependency to expand</span>
								{/if}
							</div>
						{/if}
					</div>

					{#if (data.dependencies?.length ?? 0) === 0}
						<p class="soc-subtle">No dependencies extracted yet.</p>
					{:else if dependenciesView === 'graph'}
						{#if displayedGraphModel.nodes.length === 0}
							<p class="soc-subtle">No graph data available yet for dependencies.</p>
						{:else}
							<div class="relative">
								<div
									class="repo-flow h-[74vh] overflow-hidden rounded border border-border bg-card p-2"
								>
									<DependencyGraph
										nodes={pixiGraphNodes}
										edges={pixiGraphEdges}
										rootId={displayedGraphModel.rootId}
										on:nodeclick={onGraphNodeClick}
									/>
								</div>
								{#if selectedGraphNode}
									<div class="absolute right-3 top-3 w-80 rounded border border-border bg-muted/90 p-3 text-xs shadow-lg backdrop-blur">
										<div class="mb-2 flex items-center justify-between gap-2">
											<p class="text-sm font-semibold">{selectedGraphNode.name}</p>
											<button class="soc-btn" type="button" onclick={() => (selectedGraphNodeId = '')}>Close</button>
										</div>
										<div class="space-y-1">
											<div class="flex justify-between"><span class="soc-subtle">Type</span><span>{selectedGraphNode.dependencyType}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Depth</span><span>{selectedGraphNode.depth}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Version</span><span>{selectedGraphNode.versionSpec}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Latest</span><span>{selectedGraphNode.latestVersion}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Registry</span><span>{selectedGraphNode.registry}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Manager</span><span>{selectedGraphNode.manager}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Creator</span><span>{selectedGraphNode.creator || '-'}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">License</span><span>{selectedGraphNode.license || '-'}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Last Updated</span><span>{selectedGraphNode.lastUpdated ? new Date(selectedGraphNode.lastUpdated).toLocaleDateString() : '-'}</span></div>
											{#if selectedGraphNode.description}
												<p class="pt-1 text-[11px] text-muted-foreground">{selectedGraphNode.description}</p>
											{/if}
											{#if selectedGraphNode.registryUrl}
												<a class="soc-btn mt-2 inline-block" href={selectedGraphNode.registryUrl} target="_blank" rel="noreferrer">Open Registry</a>
											{/if}
										</div>
									</div>
								{/if}
								<!--
								<div class="rounded border border-border bg-muted/10 p-3 text-xs">
									{#if selectedGraphNode}
										<p class="mb-2 text-sm font-semibold">{selectedGraphNode.name}</p>
										<div class="space-y-1">
											<div class="flex justify-between"><span class="soc-subtle">Type</span><span>{selectedGraphNode.dependencyType}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Depth</span><span>{selectedGraphNode.depth}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Version</span><span>{selectedGraphNode.versionSpec}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Latest</span><span>{selectedGraphNode.latestVersion}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Registry</span><span>{selectedGraphNode.registry}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Manager</span><span>{selectedGraphNode.manager}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Creator</span><span>{selectedGraphNode.creator || '-'}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">License</span><span>{selectedGraphNode.license || '-'}</span></div>
											<div class="flex justify-between"><span class="soc-subtle">Last Updated</span><span>{selectedGraphNode.lastUpdated ? new Date(selectedGraphNode.lastUpdated).toLocaleDateString() : '-'}</span></div>
											{#if selectedGraphNode.description}
												<p class="pt-1 text-[11px] text-muted-foreground">{selectedGraphNode.description}</p>
											{/if}
											{#if selectedGraphNode.registryUrl}
												<a class="soc-btn mt-2 inline-block" href={selectedGraphNode.registryUrl} target="_blank" rel="noreferrer">Open Registry</a>
											{/if}
										</div>
									{/if}
								</div>
								-->
							</div>
						{/if}
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
