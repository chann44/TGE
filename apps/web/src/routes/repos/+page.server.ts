import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

type GitHubRepository = {
	id: number;
	name: string;
	full_name: string;
	private: boolean;
	default_branch: string;
	html_url: string;
	connected: boolean;
};

type GitHubRepositoriesResponse = {
	repositories: GitHubRepository[];
};

const API_BASE_URL = 'http://localhost:8080';

export const load: PageServerLoad = async ({ cookies, fetch }) => {
	const session = cookies.get('session');
	if (!session) {
		return { repositories: [] as GitHubRepository[] };
	}

	const response = await fetch(`${API_BASE_URL}/v1/github/repositories`, {
		headers: {
			Authorization: `Bearer ${session}`
		}
	});

	if (response.status === 401) {
		throw redirect(302, '/auth');
	}

	if (!response.ok) {
		return { repositories: [] as GitHubRepository[] };
	}

	const payload = (await response.json()) as GitHubRepositoriesResponse;
	return { repositories: payload.repositories ?? [] };
};

export const actions: Actions = {
	connect: async ({ cookies, fetch, request }) => {
		const session = cookies.get('session');
		if (!session) {
			throw redirect(302, '/auth');
		}

		const formData = await request.formData();
		const repoId = String(formData.get('repoId') ?? '').trim();
		if (!repoId) {
			return fail(400, { message: 'Missing repository id' });
		}

		const response = await fetch(`${API_BASE_URL}/v1/github/repositories/${repoId}/connect`, {
			method: 'POST',
			headers: {
				Authorization: `Bearer ${session}`
			}
		});

		if (response.status === 401) {
			throw redirect(302, '/auth');
		}

		if (!response.ok) {
			return fail(response.status, { message: 'Failed to connect repository' });
		}

		return { success: true };
	}
};
