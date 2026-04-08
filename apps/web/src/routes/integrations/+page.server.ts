import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { getApiBaseUrl } from '$lib/server/api-base';

const API_BASE_URL = getApiBaseUrl();

type Integration = {
	provider: string;
	name: string;
	status: string;
	enabled: boolean;
	connected_at?: string;
	last_error?: string;
	config?: Record<string, unknown>;
	updated_at?: string;
};

type IntegrationActivity = {
	id: number;
	provider: string;
	action: string;
	status: string;
	detail: string;
	metadata?: Record<string, unknown>;
	created_at: string;
};

const integrationCatalog = [
	{ provider: 'github', name: 'GitHub' },
	{ provider: 'slack', name: 'Slack' },
	{ provider: 'jira', name: 'Jira' },
	{ provider: 'linear', name: 'Linear' },
	{ provider: 'discord', name: 'Discord' }
] as const;

export const load: PageServerLoad = async ({ cookies, url, fetch }) => {
	const session = cookies.get('session');
	if (!session) {
		throw redirect(302, '/auth');
	}

	const headers = { Authorization: `Bearer ${session}` };
	const [integrationsRes, activitiesRes] = await Promise.all([
		fetch(`${API_BASE_URL}/v1/integrations`, { headers }),
		fetch(`${API_BASE_URL}/v1/integrations/activities?limit=200&offset=0`, { headers })
	]);

	if (integrationsRes.status === 401 || activitiesRes.status === 401) {
		throw redirect(302, '/auth');
	}

	let integrationsPayload: Integration[] = [];
	let activitiesPayload: IntegrationActivity[] = [];

	if (integrationsRes.ok) {
		const json = (await integrationsRes.json()) as { integrations?: Integration[] };
		integrationsPayload = json.integrations ?? [];
	}

	if (activitiesRes.ok) {
		const json = (await activitiesRes.json()) as { activities?: IntegrationActivity[] };
		activitiesPayload = json.activities ?? [];
	}

	const byProvider = new Map(integrationsPayload.map((item) => [item.provider, item]));
	const integrations = integrationCatalog.map((item) => {
		const connected = byProvider.get(item.provider);
		if (connected) {
			return connected;
		}
		return {
			provider: item.provider,
			name: item.name,
			status: 'disconnected',
			enabled: false,
			connected_at: '',
			last_error: ''
		};
	});

	const pageParam = Number(url.searchParams.get('page') ?? '1');
	const pageSize = 10;
	const total = activitiesPayload.length;
	const totalPages = Math.max(1, Math.ceil(total / pageSize));
	const page = Number.isFinite(pageParam) ? Math.min(Math.max(1, Math.floor(pageParam)), totalPages) : 1;
	const start = (page - 1) * pageSize;
	const activities = activitiesPayload.slice(start, start + pageSize);

	return {
		integrations,
		activities,
		pagination: {
			page,
			pageSize,
			total,
			totalPages
		}
	};
};
