import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

const API_BASE_URL = 'http://localhost:8080';

export const load: PageServerLoad = async ({ cookies, fetch, params }) => {
	const session = cookies.get('session');
	if (!session) {
		throw redirect(302, '/auth');
	}

	const response = await fetch(`${API_BASE_URL}/v1/github/repositories/${params.repoId}/scans`, {
		headers: { Authorization: `Bearer ${session}` }
	});

	if (response.status === 401) {
		throw redirect(302, '/auth');
	}

	if (!response.ok) {
		return { scans: [] };
	}

	const payload = (await response.json()) as { scans?: any[] };
	return { scans: payload.scans ?? [] };
};
