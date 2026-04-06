<script lang="ts">
	import { SeverityBadge } from '$lib/components/security';

	let { data }: { data: any } = $props();
	let filter = $state<'all' | 'critical' | 'high' | 'medium' | 'low'>('all');
	const findings = $derived((data.findings ?? []) as any[]);
	const rows = $derived(
		filter === 'all' ? findings : findings.filter((finding) => finding.severity === filter)
	);

	const fmtTime = (value: string) => {
		if (!value) return '-';
		const parsed = new Date(value);
		if (Number.isNaN(parsed.getTime())) return '-';
		return parsed.toLocaleString();
	};
</script>

<div class="soc-page">
	<div class="flex items-center gap-2">
		<h1 class="soc-page-title">Findings</h1>
		<div class="ml-auto flex gap-1">
			{#each ['all', 'critical', 'high', 'medium', 'low'] as severity}
				<button class="soc-btn" type="button" onclick={() => (filter = severity as typeof filter)}>{severity}</button>
			{/each}
		</div>
	</div>

	<section class="soc-section">
		<table class="soc-table">
			<thead><tr><th>Severity</th><th>Finding</th><th>Repository</th><th>Package</th><th>Sources</th><th>Links</th><th>Detected</th><th>Status</th></tr></thead>
			<tbody>
				{#if rows.length === 0}
					<tr><td class="soc-subtle" colspan="8">No findings available.</td></tr>
				{:else}
					{#each rows as finding}
						<tr>
							<td><SeverityBadge value={finding.severity} /></td>
							<td><a class="hover:text-primary" href={`/findings/${finding.id}`}>{finding.title || finding.advisory_id}</a></td>
							<td>{finding.repository || '-'}</td>
							<td class="text-primary">{finding.package_name}@{finding.resolved_version || finding.version_spec || '-'}</td>
							<td class="soc-subtle">{(finding.sources ?? []).join(', ') || '-'}</td>
							<td class="soc-subtle">
								{#if (finding.source_links ?? []).length > 0}
									<div class="flex flex-wrap gap-1">
										{#each finding.source_links as link}
											<a class="soc-btn" href={link.url} target="_blank" rel="noreferrer">{link.source}</a>
										{/each}
									</div>
								{:else}
									-
								{/if}
							</td>
							<td class="soc-subtle">{fmtTime(finding.created_at)}</td>
							<td class={finding.status === 'open' ? 'soc-risk-critical' : 'soc-risk-high'}>{finding.status}</td>
						</tr>
					{/each}
				{/if}
			</tbody>
		</table>
	</section>
</div>
