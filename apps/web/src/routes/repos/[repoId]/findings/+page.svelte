<script lang="ts">
	import { page } from '$app/state';
	import { SeverityBadge } from '$lib/components/security';

	let { data }: { data: any } = $props();
	const rows = $derived((data.findings ?? []) as any[]);

	const detectedLabel = (value: string) => {
		if (!value) return '-';
		const parsed = new Date(value);
		if (Number.isNaN(parsed.getTime())) return '-';
		return parsed.toLocaleString();
	};
</script>

<div class="soc-page">
	<h1 class="soc-page-title">Repository Findings</h1>
	<p class="soc-subtle">Findings for repo: {page.params.repoId}</p>
	<section class="soc-section">
		<table class="soc-table">
			<thead><tr><th>Severity</th><th>Finding</th><th>Package</th><th>Advisory</th><th>Sources</th><th>Links</th><th>Detected</th><th>Status</th></tr></thead>
			<tbody>
				{#if rows.length === 0}
					<tr><td class="soc-subtle" colspan="8">No findings for latest scan.</td></tr>
				{:else}
					{#each rows as f}
						<tr>
							<td><SeverityBadge value={f.severity} /></td>
							<td><a class="hover:text-primary" href={`/findings/${f.id}`}>{f.title || f.advisory_id}</a></td>
							<td class="text-primary">{f.package_name}@{f.resolved_version || f.version_spec || '-'}</td>
							<td class="soc-subtle">{f.advisory_id}</td>
							<td class="soc-subtle">{(f.sources ?? []).join(', ') || '-'}</td>
							<td class="soc-subtle">
								{#if (f.source_links ?? []).length > 0}
									<div class="flex flex-wrap gap-1">
										{#each f.source_links as link}
											<a class="soc-btn" href={link.url} target="_blank" rel="noreferrer">{link.source}</a>
										{/each}
									</div>
								{:else}
									-
								{/if}
							</td>
							<td class="soc-subtle">{detectedLabel(f.created_at)}</td>
							<td class={f.status === 'open' ? 'soc-risk-critical' : 'soc-risk-high'}>{f.status}</td>
						</tr>
					{/each}
				{/if}
			</tbody>
		</table>
	</section>
</div>
