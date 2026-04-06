import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

const API_BASE_URL = 'http://localhost:8080';

type PolicySummary = {
	id: number;
	name: string;
	enabled: boolean;
	repository_count: number;
	created_at: string;
	updated_at: string;
};

type PoliciesResponse = {
	items?: PolicySummary[];
};

type GitHubRepository = {
	id: number;
	name: string;
	full_name: string;
	connected: boolean;
};

type GitHubRepositoriesResponse = {
	repositories?: GitHubRepository[];
};

const has = (form: FormData, key: string) => form.get(key) !== null;

const defaultSourcesPayload = {
	registry_first: true,
	registry_max_age_days: 7,
	registry_only: false,
	osv_enabled: true,
	ghsa_enabled: true,
	ghsa_token_ref: '',
	nvd_enabled: true,
	nvd_api_key_ref: '',
	govulncheck_enabled: true,
	supply_chain_enabled: false
};

export const load: PageServerLoad = async ({ cookies, fetch }) => {
	const session = cookies.get('session');
	if (!session) {
		throw redirect(302, '/auth');
	}

	const headers = { Authorization: `Bearer ${session}` };
	const [policiesRes, reposRes] = await Promise.all([
		fetch(`${API_BASE_URL}/v1/policies`, { headers }),
		fetch(`${API_BASE_URL}/v1/github/repositories`, { headers })
	]);

	if (policiesRes.status === 401 || reposRes.status === 401) {
		throw redirect(302, '/auth');
	}

	const policiesPayload = policiesRes.ok
		? (((await policiesRes.json()) as PoliciesResponse) ?? {})
		: {};
	const reposPayload = reposRes.ok
		? (((await reposRes.json()) as GitHubRepositoriesResponse) ?? {})
		: {};

	return {
		policies: policiesPayload.items ?? [],
		repositories: (reposPayload.repositories ?? []).filter((repo) => repo.connected)
	};
};

export const actions: Actions = {
	createPolicy: async ({ cookies, fetch, request }) => {
		const session = cookies.get('session');
		if (!session) {
			throw redirect(302, '/auth');
		}

		const form = await request.formData();
		const name = String(form.get('name') ?? '').trim();
		if (!name) {
			return fail(400, {
				action: 'createPolicy',
				success: false,
				message: 'Policy name is required.'
			});
		}

		const branches = String(form.get('trigger_branches') ?? '')
			.split(',')
			.map((value) => value.trim())
			.filter(Boolean);

		const payload = {
			name,
			enabled: has(form, 'enabled'),
			triggers: [
				{
					type: String(form.get('trigger_type') ?? 'manual').trim(),
					branches,
					cron: String(form.get('trigger_cron') ?? '').trim(),
					timezone: String(form.get('trigger_timezone') ?? 'UTC').trim()
				}
			],
			sources: defaultSourcesPayload
		};

		const response = await fetch(`${API_BASE_URL}/v1/policies`, {
			method: 'POST',
			headers: {
				Authorization: `Bearer ${session}`,
				'Content-Type': 'application/json'
			},
			body: JSON.stringify(payload)
		});

		if (response.status === 401) {
			throw redirect(302, '/auth');
		}

		if (!response.ok) {
			const errorText = (await response.text()).trim();
			return fail(response.status, {
				action: 'createPolicy',
				success: false,
				message: errorText || 'Failed to create policy.'
			});
		}

		const created = (await response.json()) as { id?: number };
		if (created.id) {
			throw redirect(302, `/policies/${created.id}?created=1`);
		}

		return { action: 'createPolicy', success: true, message: 'Policy created.' };
	},

	assignPolicy: async ({ cookies, fetch, request }) => {
		const session = cookies.get('session');
		if (!session) {
			throw redirect(302, '/auth');
		}

		const form = await request.formData();
		const repoID = String(form.get('repo_id') ?? '').trim();
		const policyID = String(form.get('policy_id') ?? '').trim();

		if (!repoID || !policyID) {
			return fail(400, {
				action: 'assignPolicy',
				success: false,
				message: 'Select both repository and policy.'
			});
		}

		const response = await fetch(`${API_BASE_URL}/v1/github/repositories/${repoID}/policy`, {
			method: 'PUT',
			headers: {
				Authorization: `Bearer ${session}`,
				'Content-Type': 'application/json'
			},
			body: JSON.stringify({ policy_id: Number(policyID) })
		});

		if (response.status === 401) {
			throw redirect(302, '/auth');
		}

		if (!response.ok) {
			const errorText = (await response.text()).trim();
			return fail(response.status, {
				action: 'assignPolicy',
				success: false,
				message: errorText || 'Failed to assign policy.'
			});
		}

		return { action: 'assignPolicy', success: true, message: 'Policy assigned to repository.' };
	},

	deletePolicy: async ({ cookies, fetch, request }) => {
		const session = cookies.get('session');
		if (!session) {
			throw redirect(302, '/auth');
		}

		const form = await request.formData();
		const policyID = String(form.get('policy_id') ?? '').trim();
		if (!policyID) {
			return fail(400, {
				action: 'deletePolicy',
				success: false,
				message: 'Policy id is required.'
			});
		}

		const response = await fetch(`${API_BASE_URL}/v1/policies/${policyID}`, {
			method: 'DELETE',
			headers: {
				Authorization: `Bearer ${session}`
			}
		});

		if (response.status === 401) {
			throw redirect(302, '/auth');
		}

		if (!response.ok) {
			const errorText = (await response.text()).trim();
			return fail(response.status, {
				action: 'deletePolicy',
				success: false,
				message: errorText || 'Failed to delete policy.'
			});
		}

		return { action: 'deletePolicy', success: true, message: 'Policy deleted.' };
	}
};
