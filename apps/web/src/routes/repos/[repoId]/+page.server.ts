import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

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
		dependency_type?: string;
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
	sync_status?: string;
	sync_error?: string;
	last_synced_at?: string;
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
			dependencies: [] as RepositoryDependency[],
			syncStatus: '',
			syncError: '',
			lastSyncedAt: ''
		};
	}

	if (!repoResponse.ok) {
		return {
			repo: null,
			dependencyFiles: [] as DependencyFile[],
			dependencies: [] as RepositoryDependency[],
			syncStatus: '',
			syncError: '',
			lastSyncedAt: ''
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
			dependencies: dependenciesPayload?.dependencies ?? [],
			syncStatus: dependenciesPayload?.sync_status ?? '',
			syncError: dependenciesPayload?.sync_error ?? '',
			lastSyncedAt: dependenciesPayload?.last_synced_at ?? ''
		};
	}

	const depFilesPayload = (await depFilesResponse.json()) as DependencyFilesResponse;
	return {
		repo,
		dependencyFiles: depFilesPayload.files ?? [],
		dependencies: dependenciesPayload?.dependencies ?? [],
		syncStatus: dependenciesPayload?.sync_status ?? '',
		syncError: dependenciesPayload?.sync_error ?? '',
		lastSyncedAt: dependenciesPayload?.last_synced_at ?? ''
	};
};

export const actions: Actions = {
	fetchDeps: async ({ cookies, fetch, params }) => {
		const session = cookies.get('session');
		if (!session) {
			throw redirect(302, '/auth');
		}

		const response = await fetch(
			`${API_BASE_URL}/v1/github/repositories/${params.repoId}/dependencies/fetch`,
			{
				method: 'POST',
				headers: {
					Authorization: `Bearer ${session}`
				}
			}
		);

		if (response.status === 401) {
			throw redirect(302, '/auth');
		}

		if (!response.ok) {
			return fail(response.status, { message: 'Failed to enqueue dependency fetch' });
		}

		const payload = (await response.json()) as { queued?: boolean; status?: string; sync_id?: number };
		return {
			queued: payload.queued === true,
			syncStatus: payload.status ?? 'queued',
			syncId: payload.sync_id ?? null,
			action: 'fetchDeps',
			message:
				payload.queued === true
					? null
					: 'Dependencies already fetched. Skipping re-fetch.'
		};
	},

	runScan: async ({ cookies, fetch, params }) => {
		const session = cookies.get('session');
		if (!session) {
			throw redirect(302, '/auth');
		}

		const response = await fetch(`${API_BASE_URL}/v1/github/repositories/${params.repoId}/scans/run`, {
			method: 'POST',
			headers: {
				Authorization: `Bearer ${session}`
			}
		});

		if (response.status === 401) {
			throw redirect(302, '/auth');
		}

		if (!response.ok) {
			const message = (await response.text()).trim();
			return fail(response.status, {
				action: 'runScan',
				success: false,
				message: message || 'Failed to queue scan run.'
			});
		}

		const payload = (await response.json()) as { queued?: boolean; status?: string; scan_id?: number };
		return {
			action: 'runScan',
			success: true,
			queued: payload.queued === true,
			scanStatus: payload.status ?? 'queued',
			scanId: payload.scan_id ?? null,
			message: payload.queued === true ? 'Scan run queued.' : 'Scan request accepted.'
		};
	}
};
