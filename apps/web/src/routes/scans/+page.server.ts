import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

const API_BASE_URL = 'http://localhost:8080';

type ScanRun = {
	id: number;
	repository: string;
	repository_id: number;
	policy: string;
	trigger: string;
	status: string;
	duration?: string;
	findings_total: number;
	started_at: string;
	finished_at?: string;
};

export const load: PageServerLoad = async ({ cookies, fetch }) => {
	const session = cookies.get('session');
	if (!session) {
		throw redirect(302, '/auth');
	}

	const response = await fetch(`${API_BASE_URL}/v1/scans`, {
		headers: { Authorization: `Bearer ${session}` }
	});

	if (response.status === 401) {
		throw redirect(302, '/auth');
	}

	if (!response.ok) {
		return { scans: [] as ScanRun[] };
	}

	const payload = (await response.json()) as { scans?: ScanRun[] };
	return {
		scans: payload.scans ?? []
	};
};
