<script lang="ts">
	import { EcoBadge, StatCard, StatusBadge } from '$lib/components/security';

	let { data }: { data: any } = $props();
	const scans = $derived((data.scans ?? []) as any[]);
	const totalScans = $derived(scans.length);
	const failedScans = $derived(scans.filter((scan: any) => scan.status === 'failed').length);
	const prScans = $derived(scans.filter((scan: any) => scan.trigger === 'pull_request').length);
</script>

<div class="soc-page">
	<div class="flex items-center justify-between">
		<h1 class="soc-page-title">Scan History</h1>
		<button class="soc-btn-primary" type="button">Trigger Scan</button>
	</div>

	<section class="soc-grid-4">
		<StatCard label="Total Scans" value={totalScans} />
		<StatCard label="Avg Duration" value={totalScans > 0 ? 'see rows' : '-'} />
		<StatCard label="Failed" value={failedScans} tone="soc-risk-high" />
		<StatCard label="PR Scans" value={prScans} />
	</section>

	<section class="soc-section">
		<table class="soc-table">
			<thead><tr><th>Scan ID</th><th>Repository</th><th>Policy</th><th>Trigger</th><th>Duration</th><th>Findings</th><th>Status</th></tr></thead>
			<tbody>
				{#if scans.length === 0}
					<tr><td class="soc-subtle" colspan="7">No scans yet.</td></tr>
				{:else}
				{#each scans as s}
					<tr class="soc-table-row-link">
						<td class="text-primary"><a href={`/scans/${s.id}`}>{s.id}</a></td>
						<td>{s.repository}</td>
						<td class="soc-subtle">{s.policy || 'Unassigned'}</td>
						<td><EcoBadge value={s.trigger} /></td>
						<td class="soc-subtle">{s.duration || '-'}</td>
						<td>{s.findings_total}</td>
						<td><StatusBadge value={s.status} /></td>
					</tr>
				{/each}
				{/if}
			</tbody>
		</table>
	</section>
</div>
