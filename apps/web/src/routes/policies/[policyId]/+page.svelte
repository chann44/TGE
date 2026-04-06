<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';

	let { data, form } = $props();
	const pageData = $derived(data as any);
	const policy = $derived(pageData.policy);
	const formData = $derived(form as any);
	let editPolicyDialogOpen = $state(false);

	$effect(() => {
		if (formData?.action === 'save' && formData?.success === false) {
			editPolicyDialogOpen = true;
		}
	});
</script>

<div class="soc-page">
	<h1 class="soc-page-title">Policy Details</h1>
	{#if pageData.flashMessage}
		<section class="soc-section border border-emerald-300 bg-emerald-50 p-3 text-xs text-emerald-800">
			{pageData.flashMessage}
		</section>
	{/if}
	{#if formData?.message}
		<section
			class={`soc-section p-3 text-xs ${
				formData?.success === false
					? 'border border-rose-300 bg-rose-50 text-rose-800'
					: 'border border-emerald-300 bg-emerald-50 text-emerald-800'
			}`}
		>
			{formData.message}
		</section>
	{/if}

	{#if policy}
		<section class="soc-grid-2">
			<section class="soc-section p-3 text-xs">
				<p class="mb-2 text-sm font-semibold">Configuration</p>
				<div class="space-y-2">
					<div class="flex items-center justify-between"><span class="soc-subtle">Name</span><span>{policy.name}</span></div>
					<div class="flex items-center justify-between"><span class="soc-subtle">Enabled</span><span>{policy.enabled ? 'yes' : 'no'}</span></div>
					<div class="flex items-center justify-between">
						<span class="soc-subtle">Primary Trigger</span>
						<span>{policy.triggers?.[0]?.type ?? 'manual'}</span>
					</div>
					<div class="flex items-center justify-between"><span class="soc-subtle">OSV</span><span>{policy.sources.osv_enabled ? 'on' : 'off'}</span></div>
					<div class="flex items-center justify-between"><span class="soc-subtle">GHSA</span><span>{policy.sources.ghsa_enabled ? 'on' : 'off'}</span></div>
					<div class="flex items-center justify-between"><span class="soc-subtle">NVD</span><span>{policy.sources.nvd_enabled ? 'on' : 'off'}</span></div>
				</div>

				<Dialog.Root bind:open={editPolicyDialogOpen}>
					<Dialog.Trigger class="soc-btn-primary mt-3" type="button">Edit Policy</Dialog.Trigger>
					<Dialog.Content class="max-h-[90vh] max-w-4xl overflow-y-auto">
						<Dialog.Header>
							<Dialog.Title>Edit Policy</Dialog.Title>
							<Dialog.Description>Update policy settings for this policy profile.</Dialog.Description>
						</Dialog.Header>

						<form method="POST" action="?/save" class="space-y-4 text-xs">
							<section class="rounded border border-border bg-muted/20 p-3">
								<p class="text-sm font-semibold">Basic</p>
								<p class="soc-subtle mb-2 text-[11px]">Update policy identity and top-level behavior.</p>
								<div class="grid grid-cols-2 gap-2">
									<label class="col-span-2" for="name"
										>Policy Name
										<input
											id="name"
											name="name"
											type="text"
											class="mt-1 w-full rounded border border-border bg-background px-2 py-1"
											value={policy.name}
											required
										/>
									</label>
									<label><input type="checkbox" name="enabled" checked={policy.enabled} /> enabled</label>
								</div>
							</section>

							<section class="rounded border border-border bg-muted/20 p-3">
								<p class="text-sm font-semibold">Trigger</p>
								<p class="soc-subtle mb-2 text-[11px]">Define when scans run.</p>
								<div class="grid grid-cols-2 gap-2">
									<label
										>trigger
										<select name="trigger_type" class="mt-1 w-full rounded border border-border bg-background px-2 py-1">
											<option value={policy.triggers?.[0]?.type ?? 'manual'}>{policy.triggers?.[0]?.type ?? 'manual'}</option>
											<option value="manual">manual</option>
											<option value="push">push</option>
											<option value="pull_request">pull_request</option>
											<option value="schedule">schedule</option>
										</select>
									</label>
									<label
										>timezone
										<input
											name="trigger_timezone"
											type="text"
											class="mt-1 w-full rounded border border-border bg-background px-2 py-1"
											value={policy.triggers?.[0]?.timezone ?? 'UTC'}
										/>
									</label>
									<label class="col-span-2"
										>cron
										<input
											name="trigger_cron"
											type="text"
											class="mt-1 w-full rounded border border-border bg-background px-2 py-1"
											value={policy.triggers?.[0]?.cron ?? ''}
										/>
									</label>
									<label class="col-span-2"
										>branches (comma separated)
										<input
											name="trigger_branches"
											type="text"
											class="mt-1 w-full rounded border border-border bg-background px-2 py-1"
											value={(policy.triggers?.[0]?.branches ?? []).join(',')}
										/>
									</label>
								</div>
							</section>

							<section class="rounded border border-border bg-muted/20 p-3 text-[11px]">
								<p class="font-semibold">Other settings</p>
								<p class="soc-subtle mt-1">
									Sources, SAST, and registry settings stay unchanged in this modal and can remain configured through backend policy fields.
								</p>
							</section>

							<div class="flex items-center justify-end gap-2">
								<Dialog.Close class="soc-btn" type="button">Cancel</Dialog.Close>
								<button class="soc-btn-primary" type="submit">Save</button>
							</div>
						</form>
					</Dialog.Content>
				</Dialog.Root>

				<div class="mt-4 rounded border border-border bg-muted/20 p-3">
					<p class="mb-2 text-sm font-semibold">Scan Source Health</p>
					<div class="space-y-2 text-[11px]">
						<div class="flex items-center justify-between">
							<span class="soc-subtle">OSV</span>
							<span class={policy.sources.osv_enabled ? 'text-emerald-700' : 'soc-subtle'}>
								{policy.sources.osv_enabled ? 'enabled' : 'disabled'}
							</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="soc-subtle">GHSA</span>
							<span class={policy.sources.ghsa_enabled ? 'text-emerald-700' : 'soc-subtle'}>{policy.sources.ghsa_enabled ? 'enabled' : 'disabled'}</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="soc-subtle">NVD</span>
							<span class={policy.sources.nvd_enabled ? 'text-emerald-700' : 'soc-subtle'}>{policy.sources.nvd_enabled ? 'enabled' : 'disabled'}</span>
						</div>
						<p class="rounded border border-sky-200 bg-sky-50 px-2 py-1 text-sky-800">
							Sources remain valid without credentials. For higher throughput, add keys where supported:
							<a class="ml-1 underline" href="https://docs.github.com/en/rest/security-advisories/global-advisories#get-a-global-security-advisory" target="_blank" rel="noreferrer">GHSA token</a>,
							<a class="ml-1 underline" href="https://nvd.nist.gov/developers/request-an-api-key" target="_blank" rel="noreferrer">NVD API key</a>.
							OSV does not require a key
							(<a class="underline" href="https://google.github.io/osv.dev/api/" target="_blank" rel="noreferrer">docs</a>).
						</p>
					</div>
				</div>
			</section>

			<section class="soc-section p-3 text-xs">
				<p class="mb-2 font-semibold">Assigned Repositories</p>
				<form method="POST" action="?/assignRepo" class="mb-3 flex items-center gap-2">
					<select class="rounded border border-border bg-background px-2 py-1" name="repo_id" required>
						<option value="">Select connected repo</option>
						{#each pageData.repositories as repo}
							<option value={repo.id}>{repo.full_name}</option>
						{/each}
					</select>
					<button class="soc-btn" type="submit">Assign</button>
				</form>

				<form method="POST" action="?/runScan" class="mb-3 flex items-center gap-2">
					<select class="rounded border border-border bg-background px-2 py-1" name="repo_id" required>
						<option value="">Select assigned repo to scan</option>
						{#each (policy.repositories ?? []) as repo}
							<option value={repo.repository_id}>{repo.full_name}</option>
						{/each}
					</select>
					<button class="soc-btn-primary" type="submit">Run Scan</button>
				</form>

				{#if (policy.repositories ?? []).length === 0}
					<p class="soc-subtle">No repositories assigned.</p>
				{:else}
					<table class="soc-table">
						<thead><tr><th>Repository</th><th>Assigned</th><th>Action</th></tr></thead>
						<tbody>
							{#each (policy.repositories ?? []) as repo}
								<tr>
									<td>{repo.full_name}</td>
									<td class="soc-subtle">{repo.assigned_at ? new Date(repo.assigned_at).toLocaleString() : '-'}</td>
									<td>
										<form method="POST" action="?/unassignRepo">
											<input type="hidden" name="repo_id" value={repo.repository_id} />
											<button class="soc-btn" type="submit">Unassign</button>
										</form>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}

				<form method="POST" action="?/deletePolicy" class="mt-4">
					<button class="soc-btn" type="submit">Delete Policy</button>
				</form>
			</section>
		</section>
	{:else}
		<section class="soc-section p-4 text-sm text-muted-foreground">Policy not found.</section>
	{/if}
</div>
