<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';

	let { data, form } = $props();
	const pageData = $derived(data as any);
	const policy = $derived(pageData.policy);
	const sourceHealth = $derived(pageData.sourceHealth ?? {});
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

							<section class="rounded border border-border bg-muted/20 p-3">
								<p class="text-sm font-semibold">Sources</p>
								<p class="soc-subtle mb-2 text-[11px]">Configure advisory source providers.</p>
								<div class="grid grid-cols-2 gap-2">
									<label><input type="checkbox" name="registry_first" checked={policy.sources.registry_first} /> registry first</label>
									<label><input type="checkbox" name="registry_only" checked={policy.sources.registry_only} /> registry only</label>
									<label><input type="checkbox" name="osv_enabled" checked={policy.sources.osv_enabled} /> osv</label>
									<label><input type="checkbox" name="ghsa_enabled" checked={policy.sources.ghsa_enabled} /> ghsa</label>
									<label><input type="checkbox" name="nvd_enabled" checked={policy.sources.nvd_enabled} /> nvd</label>
									<label><input type="checkbox" name="govulncheck_enabled" checked={policy.sources.govulncheck_enabled} /> govulncheck</label>
									<label class="col-span-2"
										>registry max age days
										<input
											name="registry_max_age_days"
											type="number"
											min="1"
											class="mt-1 w-full rounded border border-border bg-background px-2 py-1"
											value={policy.sources.registry_max_age_days}
										/>
									</label>
									<label class="col-span-2"
										>GHSA token ref
										<input
											name="ghsa_token_ref"
											type="text"
											class="mt-1 w-full rounded border border-border bg-background px-2 py-1"
											value={policy.sources.ghsa_token_ref}
										/>
									</label>
									<label class="col-span-2"
										>NVD key ref
										<input
											name="nvd_api_key_ref"
											type="text"
											class="mt-1 w-full rounded border border-border bg-background px-2 py-1"
											value={policy.sources.nvd_api_key_ref}
										/>
									</label>
								</div>
							</section>

							<details class="rounded border border-border bg-muted/20 p-3" open>
								<summary class="cursor-pointer text-sm font-semibold">SAST + AI</summary>
								<p class="soc-subtle my-2 text-[11px]">Optional static analysis and AI controls.</p>
								<div class="grid grid-cols-2 gap-2">
									<label><input type="checkbox" name="sast_enabled" checked={policy.sast.enabled} /> sast enabled</label>
									<label><input type="checkbox" name="patterns_enabled" checked={policy.sast.patterns_enabled} /> patterns enabled</label>
									<label
										>Rulesets
										<input
											name="rulesets"
											type="text"
											class="mt-1 w-full rounded border border-border bg-background px-2 py-1"
											value={(policy.sast.rulesets ?? ['default']).join(',')}
										/>
									</label>
									<label
										>Min severity
										<select name="min_severity" class="mt-1 w-full rounded border border-border bg-background px-2 py-1">
											<option value={policy.sast.min_severity}>{policy.sast.min_severity}</option>
											<option value="low">low</option>
											<option value="medium">medium</option>
											<option value="high">high</option>
											<option value="critical">critical</option>
										</select>
									</label>
									<label class="col-span-2"
										>Exclude paths
										<input
											name="exclude_paths"
											type="text"
											class="mt-1 w-full rounded border border-border bg-background px-2 py-1"
											value={(policy.sast.exclude_paths ?? []).join(',')}
										/>
									</label>
									<label><input type="checkbox" name="ai_enabled" checked={policy.sast.ai_enabled} /> ai enabled</label>
									<label><input type="checkbox" name="ai_reachability" checked={policy.sast.ai_reachability} /> ai reachability</label>
									<label><input type="checkbox" name="ai_suggest_fix" checked={policy.sast.ai_suggest_fix} /> ai suggest fix</label>
									<label
										>AI max files per scan
										<input
											name="ai_max_files_per_scan"
											type="number"
											min="1"
											class="mt-1 w-full rounded border border-border bg-background px-2 py-1"
											value={policy.sast.ai_max_files_per_scan}
										/>
									</label>
								</div>
							</details>

							<details class="rounded border border-border bg-muted/20 p-3">
								<summary class="cursor-pointer text-sm font-semibold">Registry Push/Pull</summary>
								<p class="soc-subtle my-2 text-[11px]">Publish and pull signed reports from a registry.</p>
								<div class="grid grid-cols-2 gap-2">
									<label><input type="checkbox" name="push_enabled" checked={policy.registry.push_enabled} /> push enabled</label>
									<label><input type="checkbox" name="pull_enabled" checked={policy.registry.pull_enabled} /> pull enabled</label>
									<label
										>Push URL
										<input name="push_url" type="text" class="mt-1 w-full rounded border border-border bg-background px-2 py-1" value={policy.registry.push_url} />
									</label>
									<label
										>Push signing key ref
										<input
											name="push_signing_key_ref"
											type="text"
											class="mt-1 w-full rounded border border-border bg-background px-2 py-1"
											value={policy.registry.push_signing_key_ref}
										/>
									</label>
									<label
										>Pull URL
										<input name="pull_url" type="text" class="mt-1 w-full rounded border border-border bg-background px-2 py-1" value={policy.registry.pull_url} />
									</label>
									<label
										>Pull trusted keys
										<input
											name="pull_trusted_keys"
											type="text"
											class="mt-1 w-full rounded border border-border bg-background px-2 py-1"
											value={(policy.registry.pull_trusted_keys ?? []).join(',')}
										/>
									</label>
									<label
										>Pull max age days
										<input
											name="pull_max_age_days"
											type="number"
											min="1"
											class="mt-1 w-full rounded border border-border bg-background px-2 py-1"
											value={policy.registry.pull_max_age_days}
										/>
									</label>
								</div>
							</details>

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
							<span class={sourceHealth?.osv?.enabled ? 'text-emerald-700' : 'soc-subtle'}>
								{sourceHealth?.osv?.enabled ? 'enabled' : 'disabled'}
							</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="soc-subtle">GHSA</span>
							<span
								class={sourceHealth?.ghsa?.enabled
									? sourceHealth?.ghsa?.configured
										? 'text-emerald-700'
										: 'text-rose-700'
									: 'soc-subtle'}
							>
								{#if sourceHealth?.ghsa?.enabled}
									{sourceHealth?.ghsa?.configured
										? `configured (${sourceHealth?.ghsa?.configuredBy})`
										: 'enabled, missing token'}
								{:else}
									disabled
								{/if}
							</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="soc-subtle">NVD</span>
							<span
								class={sourceHealth?.nvd?.enabled
									? sourceHealth?.nvd?.configured
										? 'text-emerald-700'
										: 'text-rose-700'
									: 'soc-subtle'}
							>
								{#if sourceHealth?.nvd?.enabled}
									{sourceHealth?.nvd?.configured
										? `configured (${sourceHealth?.nvd?.configuredBy})`
										: 'enabled, missing api key'}
								{:else}
									disabled
								{/if}
							</span>
						</div>
						{#if (sourceHealth?.ghsa?.enabled && !sourceHealth?.ghsa?.configured) || (sourceHealth?.nvd?.enabled && !sourceHealth?.nvd?.configured)}
							<p class="rounded border border-rose-200 bg-rose-50 px-2 py-1 text-rose-700">
								Some enabled sources are missing credentials. Add `ghsa_token_ref` / `nvd_api_key_ref` in this policy or set `GHSA_API_TOKEN` / `NVD_API_KEY` in server env.
							</p>
						{/if}
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
