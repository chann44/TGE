<script lang="ts">
	let { data, form }: { data: any; form: any } = $props();

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

	const integration = $derived(data.integration);
	const activities = $derived((data.activities ?? []) as any[]);
	const repositories = $derived((data.repositories ?? []) as any[]);
	const linearTeams = $derived((form?.linear_teams ?? []) as Array<{ id: string; key: string; name: string }>);
	const linearTokenValue = $derived(
		String(form?.linear_api_token ?? integration.config?.api_token ?? '').trim()
	);
	const linearTeamValue = $derived(
		String(form?.linear_team_id ?? integration.config?.team_id ?? '').trim()
	);

	const requiredFields = $derived.by(() => {
			switch (integration.provider) {
			case 'slack':
				return ['webhook_url'];
			case 'discord':
				return ['webhook_url'];
			case 'jira':
				return ['jira_base_url', 'jira_email', 'jira_api_token', 'jira_project_key'];
			case 'linear':
				return ['linear_api_token'];
			case 'github':
				return ['github app installation', 'at least one connected repository'];
			default:
				return [];
		}
	});
</script>

<div class="soc-page">
	<div class="flex items-center justify-between gap-2">
		<div>
			<h1 class="soc-page-title">{integration.name} Integration</h1>
			<p class="soc-subtle">Detailed status, routing config, and recent activity.</p>
		</div>
		<a class="soc-btn" href="/integrations">Back to Integrations</a>
	</div>

	<section class="soc-section p-3 text-xs">
		<div class="mb-2 flex items-center justify-between">
			<p class="text-sm font-semibold">Connection Status</p>
			<span class={`soc-badge ${statusClass(integration.status)}`}>{integration.status}</span>
		</div>
		<div class="grid gap-2 md:grid-cols-2">
			<div class="rounded border border-border bg-background px-2 py-1.5">
				<p class="soc-subtle">Provider</p>
				<p class="uppercase">{integration.provider}</p>
			</div>
			<div class="rounded border border-border bg-background px-2 py-1.5">
				<p class="soc-subtle">Connected At</p>
				<p>{integration.connected_at ? new Date(integration.connected_at).toLocaleString() : '-'}</p>
			</div>
			<div class="rounded border border-border bg-background px-2 py-1.5">
				<p class="soc-subtle">Enabled</p>
				<p>{integration.enabled ? 'yes' : 'no'}</p>
			</div>
			<div class="rounded border border-border bg-background px-2 py-1.5">
				<p class="soc-subtle">Last Error</p>
				<p>{integration.last_error || '-'}</p>
			</div>
		</div>
	</section>

	<section class="soc-section p-3 text-xs">
		<p class="mb-2 text-sm font-semibold">Connect / Update Integration</p>
		<p class="soc-subtle mb-2">Required setup:</p>
		<ul class="mb-3 list-disc space-y-1 pl-4 text-xs">
			{#each requiredFields as field}
				<li>{field}</li>
			{/each}
		</ul>
		<form method="POST" action="?/connect" class="grid gap-2 md:grid-cols-2">
			<label>
				<p class="soc-subtle mb-1">Display name</p>
				<input class="soc-input" name="name" value={integration.name} placeholder="Integration name" />
			</label>
			<label class="flex items-end gap-2 pb-2">
				<input type="checkbox" name="enabled" checked={integration.enabled} />
				<span>Enabled</span>
			</label>

			{#if integration.provider === 'slack' || integration.provider === 'discord'}
				<label class="md:col-span-2">
					<p class="soc-subtle mb-1">Webhook URL</p>
					<input
						class="soc-input"
						name="webhook_url"
						placeholder="https://hooks..."
						value={integration.config?.webhook_url ?? ''}
					/>
				</label>
			{/if}

			{#if integration.provider === 'jira'}
				<label>
					<p class="soc-subtle mb-1">Jira Base URL</p>
					<input
						class="soc-input"
						name="jira_base_url"
						placeholder="https://your-team.atlassian.net"
						value={integration.config?.base_url ?? ''}
					/>
				</label>
				<label>
					<p class="soc-subtle mb-1">Jira Email</p>
					<input class="soc-input" name="jira_email" value={integration.config?.email ?? ''} />
				</label>
				<label>
					<p class="soc-subtle mb-1">Jira API Token</p>
					<input class="soc-input" name="jira_api_token" placeholder="Paste API token" />
					{#if integration.config?.api_token}
						<p class="soc-subtle mt-1">
							Current token: {integration.config.api_token} (masked). Leave blank to keep it.
						</p>
					{/if}
				</label>
				<label>
					<p class="soc-subtle mb-1">Default Project Key</p>
					<input
						class="soc-input"
						name="jira_project_key"
						placeholder="SEC"
						value={integration.config?.project_key ?? ''}
					/>
				</label>
			{/if}

			{#if integration.provider === 'linear'}
				<label>
					<p class="soc-subtle mb-1">Linear API Token</p>
					<input
						class="soc-input"
						name="linear_api_token"
						placeholder="Paste API token"
						value={linearTokenValue}
					/>
					{#if integration.config?.api_token}
						<p class="soc-subtle mt-1">
							Current token: {integration.config.api_token} (masked). Leave blank to keep it.
						</p>
					{/if}
				</label>
				<label>
					<p class="soc-subtle mb-1">Linear Team ID</p>
					<input
						class="soc-input"
						name="linear_team_id"
						placeholder="team_xxx"
						value={linearTeamValue}
					/>
					{#if linearTeams.length > 0}
						<p class="soc-subtle mt-1">
							Available teams:
							{#each linearTeams as team, idx}
								{#if idx > 0}, {/if}{team.name || team.key || team.id} ({team.id})
							{/each}
						</p>
					{/if}
				</label>
			{/if}

			<div class="md:col-span-2">
				<div class="flex flex-wrap items-center gap-2">
					<button class="soc-btn-primary" type="submit">Save Integration</button>
					<button class="soc-btn" type="submit" formaction="?/runTest">Run Test</button>
					{#if integration.provider === 'linear'}
						<button class="soc-btn" type="submit" formaction="?/fetchLinearTeams">Fetch Team ID</button>
					{/if}
				</div>
			</div>
		</form>
	</section>

	{#if integration.provider === 'github' && repositories.length === 0}
		<p class="text-xs text-amber-700">
			GitHub tests require at least one connected repository. Connect a repository in the Repos page first.
		</p>
	{/if}

	{#if form?.message}
		<p class={`text-xs ${form?.success ? 'text-emerald-700' : 'text-rose-700'}`}>{form.message}</p>
	{/if}

	<section class="soc-section">
		<div class="soc-section-head">
			<p class="soc-section-label">Activity</p>
		</div>
		<table class="soc-table">
			<thead>
				<tr>
					<th>Time</th>
					<th>Action</th>
					<th>Status</th>
					<th>Details</th>
				</tr>
			</thead>
			<tbody>
				{#if activities.length === 0}
					<tr><td colspan="4" class="soc-subtle">No activity captured yet.</td></tr>
				{:else}
					{#each activities as activity}
						<tr>
							<td class="soc-subtle">{new Date(activity.created_at).toLocaleString()}</td>
							<td>{activity.action}</td>
							<td><span class={`soc-badge ${resultClass(activity.status)}`}>{activity.status}</span></td>
							<td>{activity.detail}</td>
						</tr>
					{/each}
				{/if}
			</tbody>
		</table>
	</section>
</div>
