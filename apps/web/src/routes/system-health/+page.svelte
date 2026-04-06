<script lang="ts">
	import { StatCard } from '$lib/components/security';
	import { onMount } from 'svelte';

	let { data }: { data: any } = $props();

	const summary = $derived(data.summary ?? {});
	const services = $derived(data.services ?? []);
	const queues = $derived(data.queues ?? []);
	const logs = $derived(data.logs ?? []);
	const nextCursor = $derived(data.nextCursor ?? 0);
	const filters = $derived(data.filters ?? { service: '', level: '' });

	let liveLogs = $state<any[]>([]);
	let streamStatus = $state<'connecting' | 'live' | 'reconnecting' | 'offline'>('connecting');

	$effect(() => {
		liveLogs = [...(logs ?? [])];
	});

	onMount(() => {
		const newestCursor = Number(liveLogs?.[0]?.id ?? 0) || 0;
		const params = new URLSearchParams();
		if ((filters.service ?? '').trim()) {
			params.set('service', filters.service.trim());
		}
		if ((filters.level ?? '').trim()) {
			params.set('level', filters.level.trim());
		}
		if (newestCursor > 0) {
			params.set('cursor', String(newestCursor));
		}

		const streamUrl = `/system-health/logs/stream?${params.toString()}`;
		const source = new EventSource(streamUrl);
		streamStatus = 'connecting';

		source.addEventListener('log', (event) => {
			try {
				const payload = JSON.parse((event as MessageEvent).data);
				liveLogs = [payload, ...liveLogs].slice(0, 500);
				streamStatus = 'live';
			} catch {
				streamStatus = 'reconnecting';
			}
		});

		source.onopen = () => {
			streamStatus = 'live';
		};

		source.onerror = () => {
			streamStatus = 'reconnecting';
		};

		return () => {
			source.close();
			streamStatus = 'offline';
		};
	});

	const levelClass = (level: string) => {
		switch ((level ?? '').toLowerCase()) {
			case 'error':
				return 'soc-risk-critical';
			case 'warn':
				return 'soc-risk-high';
			case 'debug':
				return 'soc-subtle';
			default:
				return 'soc-risk-ok';
		}
	};

	const statusClass = (status: string) => {
		switch ((status ?? '').toLowerCase()) {
			case 'down':
				return 'soc-risk-critical';
			case 'degraded':
				return 'soc-risk-high';
			default:
				return 'soc-risk-ok';
		}
	};
</script>

<div class="soc-page">
	<h1 class="soc-page-title">System Health</h1>

	<section class="soc-grid-4">
		<StatCard
			label="Services"
			value={`${summary.services_up ?? 0}/${summary.services_total ?? 0} up`}
			tone="soc-risk-ok"
		/>
		<StatCard label="Queue Backlog" value={summary.queue_backlog ?? 0} />
		<StatCard label="Dependency Sync Throughput" value={`${summary.dependency_sync_throughput_1h ?? 0}/h`} />
		<StatCard label="Scan Throughput" value={`${summary.scan_throughput_1h ?? 0}/h`} />
		<StatCard label="Version" value={summary.version ?? 'unknown'} />
	</section>

	<section class="soc-grid-2">
		<div class="soc-section">
			<div class="soc-section-head"><p class="soc-section-label">Services</p></div>
			<table class="soc-table">
				<thead>
					<tr>
						<th>Service</th>
						<th>Status</th>
						<th>Latency</th>
						<th>Uptime</th>
					</tr>
				</thead>
				<tbody>
					{#if services.length === 0}
						<tr>
							<td colspan="4" class="soc-subtle">No service data available</td>
						</tr>
					{:else}
						{#each services as service}
							<tr>
								<td>{service.name}</td>
								<td class={statusClass(service.status)}>{service.status}</td>
								<td class="soc-subtle">{service.latency_ms}ms</td>
								<td class="soc-subtle">{service.uptime_pct}%</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<div class="soc-section">
			<div class="soc-section-head"><p class="soc-section-label">Job Queue</p></div>
			<table class="soc-table">
				<thead>
					<tr>
						<th>Job Type</th>
						<th>Pending</th>
						<th>Running</th>
						<th>Failed (24h)</th>
					</tr>
				</thead>
				<tbody>
					{#if queues.length === 0}
						<tr>
							<td colspan="4" class="soc-subtle">No queue data available</td>
						</tr>
					{:else}
						{#each queues as q}
							<tr>
								<td class="text-primary">{q.job_type}</td>
								<td class={q.pending > 0 ? 'soc-risk-high' : 'soc-subtle'}>{q.pending}</td>
								<td class={q.running > 0 ? 'soc-risk-ok' : 'soc-subtle'}>{q.running}</td>
								<td class={q.failed > 0 ? 'soc-risk-critical' : 'soc-subtle'}>{q.failed}</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
	</section>

	<section class="soc-section">
		<div class="soc-section-head">
			<p class="soc-section-label">Service Logs</p>
			<span class="soc-subtle px-3 text-[11px]">Live: {streamStatus}</span>
		</div>
		<form method="GET" class="flex flex-wrap items-end gap-2 border-b border-border p-3 text-xs">
			<label class="flex items-center gap-2">
				<span class="soc-subtle">Service</span>
				<input class="soc-input h-8 w-40" name="service" value={filters.service ?? ''} placeholder="api" />
			</label>
			<label class="flex items-center gap-2">
				<span class="soc-subtle">Level</span>
				<select class="soc-input h-8 w-32" name="level" value={filters.level ?? ''}>
					<option value="">all</option>
					<option value="info">info</option>
					<option value="warn">warn</option>
					<option value="error">error</option>
					<option value="debug">debug</option>
				</select>
			</label>
			<button class="soc-btn" type="submit">Apply</button>
		</form>
		<div class="overflow-x-auto">
			<table class="soc-table">
				<thead>
					<tr>
						<th>Time</th>
						<th>Service</th>
						<th>Level</th>
						<th>Message</th>
					</tr>
				</thead>
				<tbody>
					{#if liveLogs.length === 0}
						<tr>
							<td colspan="4" class="soc-subtle">No logs found for this filter.</td>
						</tr>
					{:else}
						{#each liveLogs as log}
							<tr>
								<td class="soc-subtle whitespace-nowrap">{log.created_at}</td>
								<td class="text-primary">{log.service}</td>
								<td class={levelClass(log.level)}>{log.level}</td>
								<td>{log.message}</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
		{#if nextCursor && nextCursor > 0}
			<div class="border-t border-border p-3 text-xs">
				<a
					class="soc-btn"
					href={`?service=${encodeURIComponent(filters.service ?? '')}&level=${encodeURIComponent(filters.level ?? '')}&cursor=${nextCursor}`}
				>
					Load More Logs
				</a>
			</div>
		{/if}
	</section>
</div>
