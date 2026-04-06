<script lang="ts">
	import { StatusBadge } from '$lib/components/security';

	let { data }: { data: any } = $props();
	const scan = $derived(data.scan);
	const findings = $derived((data.findings ?? []) as any[]);
	const logs = $derived((data.logs ?? []) as any[]);

	const fmtTime = (value: string) => {
		if (!value) return '-';
		const parsed = new Date(value);
		if (Number.isNaN(parsed.getTime())) return '-';
		return parsed.toLocaleString();
	};
</script>

<div class="soc-page">
	<h1 class="soc-page-title">Scan Details</h1>
	{#if scan}
		<section class="soc-section p-3 text-xs">
			<div class="flex justify-between">
				<span class="soc-subtle">Scan ID</span><span class="text-primary">{scan.id}</span>
			</div>
			<div class="flex justify-between">
				<span class="soc-subtle">Repository</span><span>{scan.repository}</span>
			</div>
			<div class="flex justify-between">
				<span class="soc-subtle">Policy</span><span>{scan.policy || 'Unassigned'}</span>
			</div>
			<div class="flex justify-between">
				<span class="soc-subtle">Trigger</span><span>{scan.trigger}</span>
			</div>
			<div class="flex justify-between">
				<span class="soc-subtle">Duration</span><span>{scan.duration || '-'}</span>
			</div>
			<div class="flex justify-between">
				<span class="soc-subtle">Findings</span><span>{scan.findings_total}</span>
			</div>
			<div class="flex justify-between">
				<span class="soc-subtle">Status</span><span><StatusBadge value={scan.status} /></span>
			</div>
		</section>

		<section class="soc-section mt-3 p-3 text-xs">
			<p class="mb-2 text-sm font-semibold">Findings</p>
			{#if findings.length === 0}
				<p class="soc-subtle">No findings in this scan.</p>
			{:else}
				<table class="soc-table">
					<thead><tr><th>Severity</th><th>Package</th><th>Advisory</th><th>Sources</th><th>Links</th><th>Detail</th><th>Status</th></tr></thead>
					<tbody>
						{#each findings as finding}
							<tr>
								<td>{finding.severity}</td>
								<td>{finding.package_name}@{finding.resolved_version || finding.version_spec || '-'}</td>
								<td class="text-primary">{finding.advisory_id}</td>
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
								<td><a class="soc-btn" href={`/findings/${finding.id}`}>Open</a></td>
								<td>{finding.status}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</section>

		<section class="soc-section mt-3 p-3 text-xs">
			<p class="mb-2 text-sm font-semibold">Scan Logs</p>
			{#if logs.length === 0}
				<p class="soc-subtle">No logs captured for this scan.</p>
			{:else}
				<table class="soc-table">
					<thead><tr><th>Time</th><th>Level</th><th>Directory</th><th>Message</th></tr></thead>
					<tbody>
						{#each logs as log}
							<tr>
								<td class="soc-subtle">{fmtTime(log.created_at)}</td>
								<td>{log.level}</td>
								<td class="text-primary">{log.directory_path || '-'}</td>
								<td>{log.message}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</section>
	{:else}
		<section class="soc-section p-4 text-sm text-muted-foreground">Scan not found.</section>
	{/if}
</div>
