import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

const API_BASE_URL = 'http://localhost:8080';

type SummaryResponse = {
	services_up?: number;
	services_total?: number;
	queue_backlog?: number;
	dependency_sync_throughput_1h?: number;
	scan_throughput_1h?: number;
	version?: string;
};

type ServicesResponse = {
	services?: Array<{
		key: string;
		name: string;
		status: string;
		latency_ms: number;
		uptime_pct: number;
		last_checked_at: string;
		note: string;
	}>;
};

type QueuesResponse = {
	queues?: Array<{
		queue: string;
		job_type: string;
		pending: number;
		running: number;
		failed: number;
		sampled_at: string;
	}>;
};

type LogsResponse = {
	items?: Array<{
		id: number;
		service: string;
		level: string;
		message: string;
		metadata: Record<string, unknown>;
		created_at: string;
	}>;
	next_cursor?: number;
};

export const load: PageServerLoad = async ({ cookies, fetch, url }) => {
	const session = cookies.get('session');
	if (!session) {
		throw redirect(302, '/auth');
	}

	const serviceFilter = (url.searchParams.get('service') ?? '').trim();
	const levelFilter = (url.searchParams.get('level') ?? '').trim();
	const cursor = (url.searchParams.get('cursor') ?? '').trim();

	const headers = { Authorization: `Bearer ${session}` };
	const logsQuery = new URLSearchParams();
	if (serviceFilter) logsQuery.set('service', serviceFilter);
	if (levelFilter) logsQuery.set('level', levelFilter);
	if (cursor) logsQuery.set('cursor', cursor);
	logsQuery.set('limit', '50');

	const [summaryRes, servicesRes, queuesRes, logsRes] = await Promise.all([
		fetch(`${API_BASE_URL}/v1/system-health/summary`, { headers }),
		fetch(`${API_BASE_URL}/v1/system-health/services`, { headers }),
		fetch(`${API_BASE_URL}/v1/system-health/queues`, { headers }),
		fetch(`${API_BASE_URL}/v1/system-health/logs?${logsQuery.toString()}`, { headers })
	]);

	if (summaryRes.status === 401 || servicesRes.status === 401 || queuesRes.status === 401 || logsRes.status === 401) {
		throw redirect(302, '/auth');
	}

	const summary = summaryRes.ok ? (((await summaryRes.json()) as SummaryResponse) ?? {}) : {};
	const services = servicesRes.ok ? (((await servicesRes.json()) as ServicesResponse) ?? {}) : {};
	const queues = queuesRes.ok ? (((await queuesRes.json()) as QueuesResponse) ?? {}) : {};
	const logs = logsRes.ok ? (((await logsRes.json()) as LogsResponse) ?? {}) : {};

	return {
		summary,
		services: services.services ?? [],
		queues: queues.queues ?? [],
		logs: logs.items ?? [],
		nextCursor: logs.next_cursor ?? 0,
		filters: {
			service: serviceFilter,
			level: levelFilter
		}
	};
};
