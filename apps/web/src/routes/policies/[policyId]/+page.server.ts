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
	const envGHSAConfigured = String(process.env.GHSA_API_TOKEN ?? '').trim().length > 0;
	const envNVDConfigured = String(process.env.NVD_API_KEY ?? '').trim().length > 0;
	const policyGHSAConfigured = String(policy.sources?.ghsa_token_ref ?? '').trim().length > 0;
	const policyNVDConfigured = String(policy.sources?.nvd_api_key_ref ?? '').trim().length > 0;

	return {
		policy,
		repositories: (reposPayload.repositories ?? []).filter((repo) => repo.connected),
		flashMessage: url.searchParams.get('created') === '1' ? 'Policy created successfully.' : '',
		sourceHealth: {
			osv: { enabled: policy.sources?.osv_enabled === true, configured: true },
			ghsa: {
				enabled: policy.sources?.ghsa_enabled === true,
				configured: policyGHSAConfigured || envGHSAConfigured,
				configuredBy: policyGHSAConfigured ? 'policy' : envGHSAConfigured ? 'env' : 'none'
			},
			nvd: {
				enabled: policy.sources?.nvd_enabled === true,
				configured: policyNVDConfigured || envNVDConfigured,
				configuredBy: policyNVDConfigured ? 'policy' : envNVDConfigured ? 'env' : 'none'
			}
		}
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
			sources: {
				registry_first: has(form, 'registry_first'),
				registry_max_age_days: Number(form.get('registry_max_age_days') ?? 7),
				registry_only: has(form, 'registry_only'),
				osv_enabled: has(form, 'osv_enabled'),
				ghsa_enabled: has(form, 'ghsa_enabled'),
				ghsa_token_ref: String(form.get('ghsa_token_ref') ?? '').trim(),
				nvd_enabled: has(form, 'nvd_enabled'),
				nvd_api_key_ref: String(form.get('nvd_api_key_ref') ?? '').trim(),
				govulncheck_enabled: has(form, 'govulncheck_enabled')
			},
			sast: {
				enabled: has(form, 'sast_enabled'),
				patterns_enabled: has(form, 'patterns_enabled'),
				rulesets: String(form.get('rulesets') ?? '')
					.split(',')
					.map((value) => value.trim())
					.filter(Boolean),
				min_severity: String(form.get('min_severity') ?? 'medium').trim(),
				exclude_paths: String(form.get('exclude_paths') ?? '')
					.split(',')
					.map((value) => value.trim())
					.filter(Boolean),
				ai_enabled: has(form, 'ai_enabled'),
				ai_max_files_per_scan: Number(form.get('ai_max_files_per_scan') ?? 50),
				ai_reachability: has(form, 'ai_reachability'),
				ai_suggest_fix: has(form, 'ai_suggest_fix')
			},
			registry: {
				push_enabled: has(form, 'push_enabled'),
				push_url: String(form.get('push_url') ?? '').trim(),
				push_signing_key_ref: String(form.get('push_signing_key_ref') ?? '').trim(),
				pull_enabled: has(form, 'pull_enabled'),
				pull_url: String(form.get('pull_url') ?? '').trim(),
				pull_trusted_keys: String(form.get('pull_trusted_keys') ?? '')
					.split(',')
					.map((value) => value.trim())
					.filter(Boolean),
				pull_max_age_days: Number(form.get('pull_max_age_days') ?? 7)
			}
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
