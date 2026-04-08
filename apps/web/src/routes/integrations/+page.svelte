<script lang="ts">
	import SlackLogo from '$lib/assets/integrations/slack.svg';
	import JiraLogo from '$lib/assets/integrations/jira.svg';
	import LinearLogo from '$lib/assets/integrations/linear.svg';
	import DiscordLogo from '$lib/assets/integrations/discord.svg';

	let { data }: { data: any } = $props();

	const statusClass = (status: string) => {
		switch (status) {
			case 'connected':
				return 'soc-status-ok';
			case 'error':
				return 'soc-status-info';
			default:
				return 'soc-status-fail';
		}
	};

	const resultClass = (result: string) => {
		switch (result) {
			case 'success':
				return 'soc-status-ok';
			case 'error':
				return 'soc-status-info';
			case 'failed':
				return 'soc-status-fail';
			default:
				return 'soc-status-info';
		}
	};

	const getLogoSrc = (provider: string) => {
		switch (provider) {
			case 'slack':
				return SlackLogo;
			case 'jira':
				return JiraLogo;
			case 'linear':
				return LinearLogo;
			case 'discord':
				return DiscordLogo;
			case 'github':
				return '';
			default:
				return '';
		}
	};
</script>

<div class="soc-page">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="soc-page-title">Integrations</h1>
			<p class="soc-subtle">Connect and monitor external tools for alerts and ticketing.</p>
		</div>
	</div>

	<section class="soc-grid-2">
		{#each data.integrations as integration}
			<a href={`/integrations/${integration.provider}`} class="soc-section block p-3 transition-colors hover:bg-background/40">
				<div class="flex items-center justify-between gap-2">
					<div class="flex items-center gap-2">
						<div class="flex h-8 w-8 items-center justify-center rounded-md bg-zinc-900">
							{#if getLogoSrc(integration.provider)}
								<img src={getLogoSrc(integration.provider)} alt={`${integration.name} logo`} class="h-6 w-6" />
							{:else}
								<span class="text-xs font-semibold text-zinc-200">GH</span>
							{/if}
						</div>
						<p class="text-sm font-semibold">{integration.name}</p>
					</div>
					<span class={`soc-badge ${statusClass(integration.status)}`}>{integration.status}</span>
				</div>
			</a>
		{/each}
	</section>

	<section class="soc-section">
		<div class="soc-section-head">
			<p class="soc-section-label">Recent Integration Activity</p>
			<p class="soc-subtle text-[10px]">
				{data.pagination.total} total events
			</p>
		</div>
		<table class="soc-table">
			<thead>
				<tr>
					<th>Time</th>
					<th>Provider</th>
					<th>Action</th>
					<th>Status</th>
					<th>Details</th>
				</tr>
			</thead>
			<tbody>
				{#if data.activities.length === 0}
					<tr><td colspan="6" class="soc-subtle">No activity yet.</td></tr>
				{:else}
					{#each data.activities as activity}
						<tr>
							<td class="soc-subtle">{new Date(activity.created_at).toLocaleString()}</td>
							<td class="uppercase">{activity.provider}</td>
							<td>{activity.action}</td>
							<td><span class={`soc-badge ${resultClass(activity.status)}`}>{activity.status}</span></td>
							<td>{activity.detail}</td>
						</tr>
					{/each}
				{/if}
			</tbody>
		</table>

		<div class="flex items-center justify-between border-t border-border px-2.5 py-2 text-xs">
			<p class="soc-subtle">Page {data.pagination.page} of {data.pagination.totalPages}</p>
			<div class="flex items-center gap-2">
				{#if data.pagination.page > 1}
					<a class="soc-btn" href={`/integrations?page=${data.pagination.page - 1}`}>Previous</a>
				{/if}
				{#if data.pagination.page < data.pagination.totalPages}
					<a class="soc-btn" href={`/integrations?page=${data.pagination.page + 1}`}>Next</a>
				{/if}
			</div>
		</div>
	</section>
</div>
