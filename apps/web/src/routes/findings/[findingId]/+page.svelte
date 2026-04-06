<script lang="ts">
	import { SeverityBadge } from '$lib/components/security';

	let { data }: { data: any } = $props();
	const finding = $derived(data.finding);

	const fmtTime = (value: string) => {
		if (!value) return '-';
		const parsed = new Date(value);
		if (Number.isNaN(parsed.getTime())) return '-';
		return parsed.toLocaleString();
	};
</script>

<div class="soc-page">
	<h1 class="soc-page-title">Finding Details</h1>
	{#if finding}
		<section class="soc-section p-4">
			<div class="mb-3 flex items-start gap-2">
				<SeverityBadge value={finding.severity} />
				<p class="text-sm font-semibold">{finding.title || finding.advisory_id}</p>
			</div>
			<div class="grid gap-2 text-xs md:grid-cols-3">
				<div><p class="soc-subtle">Repository</p><p>{finding.repository || '-'}</p></div>
				<div><p class="soc-subtle">Policy</p><p>{finding.policy || 'Unassigned'}</p></div>
				<div><p class="soc-subtle">Package</p><p class="text-primary">{finding.package_name}@{finding.resolved_version || finding.version_spec || '-'}</p></div>
				<div><p class="soc-subtle">Advisory</p><p>{finding.advisory_id}</p></div>
				<div><p class="soc-subtle">Detected</p><p>{fmtTime(finding.created_at)}</p></div>
				<div><p class="soc-subtle">Status</p><p>{finding.status}</p></div>
				<div><p class="soc-subtle">Manager</p><p>{finding.manager}</p></div>
				<div><p class="soc-subtle">Registry</p><p>{finding.registry}</p></div>
				<div><p class="soc-subtle">Fixed Version</p><p>{finding.fixed_version || '-'}</p></div>
			</div>
		</section>

		<section class="soc-grid-2">
			<div class="soc-section">
				<div class="soc-section-head"><p class="soc-section-label">Summary</p></div>
				<div class="space-y-2 p-3 text-xs">
					<p>{finding.summary || 'No summary available.'}</p>
					{#if (finding.aliases ?? []).length > 0}
						<p class="soc-subtle">Aliases: {(finding.aliases ?? []).join(', ')}</p>
					{/if}
				</div>
			</div>
			<div class="soc-section">
				<div class="soc-section-head"><p class="soc-section-label">Sources</p></div>
				<div class="space-y-2 p-3 text-xs">
					<p class="soc-subtle">Matched sources: {(finding.sources ?? []).join(', ') || '-'}</p>
					{#if (finding.source_links ?? []).length > 0}
						<div class="flex flex-wrap gap-2">
							{#each finding.source_links as link}
								<a class="soc-btn" href={link.url} target="_blank" rel="noreferrer">Open {link.source}</a>
							{/each}
						</div>
					{:else}
						<p class="soc-subtle">No source links available.</p>
					{/if}
				</div>
			</div>
		</section>
	{:else}
		<section class="soc-section p-4 text-sm text-muted-foreground">Finding not found.</section>
	{/if}
</div>
