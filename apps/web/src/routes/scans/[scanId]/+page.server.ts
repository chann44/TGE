import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

const API_BASE_URL = 'http://localhost:8080';

export const load: PageServerLoad = async ({ cookies, fetch, params }) => {
	const session = cookies.get('session');
	if (!session) {
		throw redirect(302, '/auth');
	}

	const response = await fetch(`${API_BASE_URL}/v1/scans/${params.scanId}`, {
		headers: { Authorization: `Bearer ${session}` }
	});

	if (response.status === 401) {
		throw redirect(302, '/auth');
	}

	if (response.status === 404) {
		return { scan: null, findings: [], logs: [] };
	}

	if (!response.ok) {
		return { scan: null, findings: [], logs: [] };
	}

	const payload = (await response.json()) as { scan?: any; findings?: any[]; logs?: any[] };
	return {
		scan: payload.scan ?? null,
		findings: payload.findings ?? [],
		logs: payload.logs ?? []
	};
};
