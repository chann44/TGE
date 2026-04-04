import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

type GitHubRepository = {
	id: number;
	name: string;
	full_name: string;
	private: boolean;
	default_branch: string;
	html_url: string;
	description: string;
	language: string;
	stargazers_count: number;
	forks_count: number;
	open_issues_count: number;
	updated_at: string;
	connected: boolean;
};

const API_BASE_URL = 'http://localhost:8080';

export const load: PageServerLoad = async ({ cookies, fetch, params }) => {
	const session = cookies.get('session');
	if (!session) {
		throw redirect(302, '/auth');
	}

	const response = await fetch(`${API_BASE_URL}/v1/github/repositories/${params.repoId}`, {
		headers: {
			Authorization: `Bearer ${session}`
		}
	});

	if (response.status === 401) {
		throw redirect(302, '/auth');
	}

	if (response.status === 404) {
		return { repo: null };
	}

	if (!response.ok) {
		return { repo: null };
	}

	const repo = (await response.json()) as GitHubRepository;
	return { repo };
};
