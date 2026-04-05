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

type DependencyFile = {
	path: string;
	file: string;
	manager: string;
	registry: string;
};

type DependencyFilesResponse = {
	repository_id: number;
	full_name: string;
	files: DependencyFile[];
};

type RepositoryDependency = {
	name: string;
	version_spec: string;
	version_specs?: string[];
	latest_version: string;
	manager: string;
	registry: string;
	scope: string;
	scopes?: string[];
	source_file: string;
	used_in_files?: string[];
	usage_count: number;
	creator: string;
	description: string;
	license: string;
	homepage: string;
	repository_url: string;
	registry_url: string;
	last_updated: string;
	dependency_graph?: Array<{
		name: string;
		parent?: string;
		depth: number;
		version_spec: string;
		latest_version: string;
		manager: string;
		registry: string;
		creator: string;
		description: string;
		license: string;
		homepage: string;
		repository_url: string;
		registry_url: string;
		last_updated: string;
	}>;
};

type DependenciesResponse = {
	repository_id: number;
	full_name: string;
	page: number;
	page_size: number;
	total: number;
	total_pages: number;
	dependencies: RepositoryDependency[];
};

const API_BASE_URL = 'http://localhost:8080';

export const load: PageServerLoad = async ({ cookies, fetch, params }) => {
	const session = cookies.get('session');
	if (!session) {
		throw redirect(302, '/auth');
	}

	const headers = {
		Authorization: `Bearer ${session}`
	};

	const [repoResponse, depFilesResponse, dependenciesResponse] = await Promise.all([
		fetch(`${API_BASE_URL}/v1/github/repositories/${params.repoId}`, { headers }),
		fetch(`${API_BASE_URL}/v1/github/repositories/${params.repoId}/dependency-files`, { headers }),
		fetch(`${API_BASE_URL}/v1/github/repositories/${params.repoId}/dependencies`, { headers })
	]);

	if (repoResponse.status === 401) {
		throw redirect(302, '/auth');
	}

	if (repoResponse.status === 404) {
		return {
			repo: null,
			dependencyFiles: [] as DependencyFile[],
			dependencies: [] as RepositoryDependency[]
		};
	}

	if (!repoResponse.ok) {
		return {
			repo: null,
			dependencyFiles: [] as DependencyFile[],
			dependencies: [] as RepositoryDependency[]
		};
	}

	const repo = (await repoResponse.json()) as GitHubRepository;
	const dependenciesPayload = dependenciesResponse.ok
		? ((await dependenciesResponse.json()) as DependenciesResponse)
		: null;

	if (!depFilesResponse.ok) {
		return {
			repo,
			dependencyFiles: [] as DependencyFile[],
			dependencies: dependenciesPayload?.dependencies ?? []
		};
	}

	const depFilesPayload = (await depFilesResponse.json()) as DependencyFilesResponse;
	return {
		repo,
		dependencyFiles: depFilesPayload.files ?? [],
		dependencies: dependenciesPayload?.dependencies ?? []
	};
};
