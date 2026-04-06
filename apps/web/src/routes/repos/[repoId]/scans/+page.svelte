<script lang="ts">
	import { EcoBadge, StatusBadge } from '$lib/components/security';
	import { page } from '$app/state';

	let { data }: { data: any } = $props();
	const rows = $derived((data.scans ?? []) as any[]);
</script>

<div class="soc-page">
	<h1 class="soc-page-title">Repository Scans</h1>
	<p class="soc-subtle">Scans for repo: {page.params.repoId}</p>
	<section class="soc-section">
		<table class="soc-table">
			<thead
				><tr
					><th>Scan ID</th><th>Policy</th><th>Trigger</th><th>Duration</th><th>Findings</th><th
						>Status</th
					></tr
				></thead
			>
			<tbody>
				{#if rows.length === 0}
					<tr><td class="soc-subtle" colspan="6">No scans yet for this repository.</td></tr>
				{:else}
				{#each rows as s}
					<tr>
						<td class="text-primary">{s.id}</td>
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
