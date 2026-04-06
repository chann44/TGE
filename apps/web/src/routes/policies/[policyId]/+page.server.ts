import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

const API_BASE_URL = 'http://localhost:8080';

type PolicyDetail = {
	id: number;
	name: string;
	enabled: boolean;
	triggers: Array<{
		type: string;
		branches?: string[];
		cron?: string;
		timezone?: string;
	}>;
	sources: {
		registry_first: boolean;
		registry_max_age_days: number;
		registry_only: boolean;
		osv_enabled: boolean;
		ghsa_enabled: boolean;
		ghsa_token_ref: string;
		nvd_enabled: boolean;
		nvd_api_key_ref: string;
		govulncheck_enabled: boolean;
	};
	repositories: Array<{
		repository_id: number;
		full_name: string;
		assigned_at: string;
	}>;
};

type RepositoriesResponse = {
	repositories?: Array<{
		id: number;
		full_name: string;
		connected: boolean;
	}>;
};

const has = (form: FormData, key: string) => form.get(key) !== null;

export const load: PageServerLoad = async ({ cookies, fetch, params, url }) => {
	const session = cookies.get('session');
	if (!session) {
		throw redirect(302, '/auth');
	}

	const headers = { Authorization: `Bearer ${session}` };
	const [policyRes, reposRes] = await Promise.all([
		fetch(`${API_BASE_URL}/v1/policies/${params.policyId}`, { headers }),
		fetch(`${API_BASE_URL}/v1/github/repositories`, { headers })
	]);

	if (policyRes.status === 401 || reposRes.status === 401) {
		throw redirect(302, '/auth');
	}

	if (policyRes.status === 404) {
		return { policy: null, repositories: [], flashMessage: '' };
	}

	if (!policyRes.ok) {
		return { policy: null, repositories: [], flashMessage: '' };
	}

	const policy = (await policyRes.json()) as PolicyDetail;
	const reposPayload = reposRes.ok ? (((await reposRes.json()) as RepositoriesResponse) ?? {}) : {};

	return {
		policy,
		repositories: (reposPayload.repositories ?? []).filter((repo) => repo.connected),
		flashMessage: url.searchParams.get('created') === '1' ? 'Policy created successfully.' : ''
	};
};

export const actions: Actions = {
	save: async ({ cookies, fetch, request, params }) => {
		const session = cookies.get('session');
		if (!session) throw redirect(302, '/auth');

		const form = await request.formData();
		const name = String(form.get('name') ?? '').trim();
		if (!name) {
			return fail(400, {
				action: 'save',
				success: false,
				message: 'Policy name is required.'
			});
		}

		const branchesRaw = String(form.get('trigger_branches') ?? '')
			.split(',')
			.map((v) => v.trim())
			.filter(Boolean);

		const currentPolicyResponse = await fetch(`${API_BASE_URL}/v1/policies/${params.policyId}`, {
			headers: { Authorization: `Bearer ${session}` }
		});
		if (currentPolicyResponse.status === 401) throw redirect(302, '/auth');
		if (!currentPolicyResponse.ok) {
			const errorText = (await currentPolicyResponse.text()).trim();
			return fail(currentPolicyResponse.status, {
				action: 'save',
				success: false,
				message: errorText || 'Failed to load current policy settings.'
			});
		}
		const currentPolicy = (await currentPolicyResponse.json()) as any;

		const payload = {
			name,
			enabled: has(form, 'enabled'),
			triggers: [
				{
					type: String(form.get('trigger_type') ?? 'manual').trim(),
					branches: branchesRaw,
					cron: String(form.get('trigger_cron') ?? '').trim(),
					timezone: String(form.get('trigger_timezone') ?? 'UTC').trim()
				}
			],
			sources: currentPolicy.sources,
			sast: currentPolicy.sast,
			registry: currentPolicy.registry
		};

		const response = await fetch(`${API_BASE_URL}/v1/policies/${params.policyId}`, {
			method: 'PUT',
			headers: {
				Authorization: `Bearer ${session}`,
				'Content-Type': 'application/json'
			},
			body: JSON.stringify(payload)
		});

		if (response.status === 401) throw redirect(302, '/auth');
		if (!response.ok) {
			const errorText = (await response.text()).trim();
			return fail(response.status, {
				action: 'save',
				success: false,
				message: errorText || 'Failed to save policy.'
			});
		}

		return { action: 'save', success: true, message: 'Policy updated.' };
	},

	assignRepo: async ({ cookies, fetch, request, params }) => {
		const session = cookies.get('session');
		if (!session) throw redirect(302, '/auth');

		const form = await request.formData();
		const repoID = String(form.get('repo_id') ?? '').trim();
		if (!repoID) {
			return fail(400, {
				action: 'assignRepo',
				success: false,
				message: 'Select a repository.'
			});
		}

		const response = await fetch(`${API_BASE_URL}/v1/github/repositories/${repoID}/policy`, {
			method: 'PUT',
			headers: {
				Authorization: `Bearer ${session}`,
				'Content-Type': 'application/json'
			},
			body: JSON.stringify({ policy_id: Number(params.policyId) })
		});

		if (response.status === 401) throw redirect(302, '/auth');
		if (!response.ok) {
			const errorText = (await response.text()).trim();
			return fail(response.status, {
				action: 'assignRepo',
				success: false,
				message: errorText || 'Failed to assign repository.'
			});
		}

		return { action: 'assignRepo', success: true, message: 'Repository assigned.' };
	},

	unassignRepo: async ({ cookies, fetch, request }) => {
		const session = cookies.get('session');
		if (!session) throw redirect(302, '/auth');

		const form = await request.formData();
		const repoID = String(form.get('repo_id') ?? '').trim();
		if (!repoID) {
			return fail(400, {
				action: 'unassignRepo',
				success: false,
				message: 'Repository is required.'
			});
		}

		const response = await fetch(`${API_BASE_URL}/v1/github/repositories/${repoID}/policy`, {
			method: 'DELETE',
			headers: { Authorization: `Bearer ${session}` }
		});

		if (response.status === 401) throw redirect(302, '/auth');
		if (!response.ok) {
			const errorText = (await response.text()).trim();
			return fail(response.status, {
				action: 'unassignRepo',
				success: false,
				message: errorText || 'Failed to unassign repository.'
			});
		}

		return { action: 'unassignRepo', success: true, message: 'Repository unassigned.' };
	},

	runScan: async ({ cookies, fetch, request, params }) => {
		const session = cookies.get('session');
		if (!session) throw redirect(302, '/auth');

		const form = await request.formData();
		const repoID = Number(String(form.get('repo_id') ?? '').trim());
		if (!repoID || repoID <= 0) {
			return fail(400, {
				action: 'runScan',
				success: false,
				message: 'Select a repository to run scan.'
			});
		}

		const response = await fetch(`${API_BASE_URL}/v1/policies/${params.policyId}/scans/run`, {
			method: 'POST',
			headers: {
				Authorization: `Bearer ${session}`,
				'Content-Type': 'application/json'
			},
			body: JSON.stringify({ repo_id: repoID })
		});

		if (response.status === 401) throw redirect(302, '/auth');
		if (!response.ok) {
			const errorText = (await response.text()).trim();
			return fail(response.status, {
				action: 'runScan',
				success: false,
				message: errorText || 'Failed to queue scan.'
			});
		}

		return { action: 'runScan', success: true, message: 'Scan run queued.' };
	},

	deletePolicy: async ({ cookies, fetch, params }) => {
		const session = cookies.get('session');
		if (!session) throw redirect(302, '/auth');

		const response = await fetch(`${API_BASE_URL}/v1/policies/${params.policyId}`, {
			method: 'DELETE',
			headers: { Authorization: `Bearer ${session}` }
		});

		if (response.status === 401) throw redirect(302, '/auth');
		if (!response.ok) {
			const errorText = (await response.text()).trim();
			return fail(response.status, {
				action: 'deletePolicy',
				success: false,
				message: errorText || 'Failed to delete policy. Unassign repos first.'
			});
		}

		throw redirect(302, '/policies');
	}
};
