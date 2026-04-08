import { error, fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { getApiBaseUrl } from '$lib/server/api-base';

const API_BASE_URL = getApiBaseUrl();

const integrationCatalog = {
	github: { provider: 'github', name: 'GitHub' },
	slack: { provider: 'slack', name: 'Slack' },
	jira: { provider: 'jira', name: 'Jira' },
	linear: { provider: 'linear', name: 'Linear' },
	discord: { provider: 'discord', name: 'Discord' }
} as const;

type Integration = {
	provider: string;
	name: string;
	status: string;
	enabled: boolean;
	connected_at?: string;
	last_error?: string;
	config?: Record<string, unknown>;
	updated_at?: string;
};

type IntegrationActivity = {
	id: number;
	provider: string;
	action: string;
	status: string;
	detail: string;
	created_at: string;
	metadata?: Record<string, unknown>;
};

type GitHubRepository = {
	id: number;
	name: string;
	full_name: string;
	connected: boolean;
};

type LinearTeam = {
	id: string;
	key: string;
	name: string;
};

export const load: PageServerLoad = async ({ cookies, params, fetch }) => {
	const session = cookies.get('session');
	if (!session) {
		throw redirect(302, '/auth');
	}

	const provider = params.provider.toLowerCase().trim();
	if (!(provider in integrationCatalog)) {
		throw error(404, 'Integration not found');
	}

	const headers = { Authorization: `Bearer ${session}` };
	const shouldLoadGitHubRepos = provider === 'github';
	const [integrationRes, activitiesRes, repositoriesRes] = await Promise.all([
		fetch(`${API_BASE_URL}/v1/integrations/${provider}`, { headers }),
		fetch(`${API_BASE_URL}/v1/integrations/activities?limit=200&offset=0`, { headers }),
		shouldLoadGitHubRepos
			? fetch(`${API_BASE_URL}/v1/github/repositories?page=1&page_size=100`, { headers })
			: Promise.resolve(null)
	]);

	if (
		integrationRes.status === 401 ||
		activitiesRes.status === 401 ||
		repositoriesRes?.status === 401
	) {
		throw redirect(302, '/auth');
	}

	let integration: Integration = {
		provider,
		name: integrationCatalog[provider as keyof typeof integrationCatalog].name,
		status: 'disconnected',
		enabled: false,
		connected_at: '',
		last_error: '',
		config: {}
	};

	if (integrationRes.ok) {
		const json = (await integrationRes.json()) as { integration?: Integration };
		if (json.integration) integration = json.integration;
	}

	let activities: IntegrationActivity[] = [];
	if (activitiesRes.ok) {
		const json = (await activitiesRes.json()) as { activities?: IntegrationActivity[] };
		activities = (json.activities ?? []).filter((item) => item.provider === provider);
	}

	let repositories: GitHubRepository[] = [];
	if (repositoriesRes?.ok) {
		const json = (await repositoriesRes.json()) as { repositories?: GitHubRepository[] };
		repositories = (json.repositories ?? []).filter((repo) => repo.connected);
	}

	return { integration, activities, repositories };
};

export const actions: Actions = {
	fetchLinearTeams: async ({ cookies, fetch, request, params }) => {
		const session = cookies.get('session');
		if (!session) {
			throw redirect(302, '/auth');
		}

		const provider = params.provider.toLowerCase().trim();
		if (provider !== 'linear') {
			return fail(400, {
				action: 'fetchLinearTeams',
				message: 'Team lookup is only supported for Linear.'
			});
		}

		const formData = await request.formData();
		const linearToken = String(formData.get('linear_api_token') ?? '').trim();
		if (!linearToken) {
			return fail(400, {
				action: 'fetchLinearTeams',
				message: 'Linear API token is required.',
				linear_api_token: '',
				linear_team_id: ''
			});
		}

		const response = await fetch(`${API_BASE_URL}/v1/integrations/linear/teams`, {
			method: 'POST',
			headers: {
				Authorization: `Bearer ${session}`,
				'Content-Type': 'application/json'
			},
			body: JSON.stringify({ linear_api_token: linearToken })
		});

		if (response.status === 401) {
			throw redirect(302, '/auth');
		}

		if (!response.ok) {
			return fail(response.status, {
				action: 'fetchLinearTeams',
				message: await response.text(),
				linear_api_token: linearToken,
				linear_team_id: ''
			});
		}

		const payload = (await response.json()) as { teams?: LinearTeam[] };
		const teams = payload.teams ?? [];
		const firstTeamID = String(teams[0]?.id ?? '').trim();

		if (!firstTeamID) {
			return fail(400, {
				action: 'fetchLinearTeams',
				message: 'No teams found for this token.',
				linear_api_token: linearToken,
				linear_team_id: ''
			});
		}

		return {
			action: 'fetchLinearTeams',
			success: true,
			message: `Found ${teams.length} team(s).`,
			linear_api_token: linearToken,
			linear_team_id: firstTeamID,
			linear_teams: teams
		};
	},

	connect: async ({ cookies, fetch, request, params }) => {
		const session = cookies.get('session');
		if (!session) {
			throw redirect(302, '/auth');
		}

		const provider = params.provider.toLowerCase().trim();
		const formData = await request.formData();

		const payload = {
			name: String(formData.get('name') ?? '').trim(),
			enabled: formData.get('enabled') === 'on',
			webhook_url: String(formData.get('webhook_url') ?? '').trim(),
			jira_base_url: String(formData.get('jira_base_url') ?? '').trim(),
			jira_email: String(formData.get('jira_email') ?? '').trim(),
			jira_api_token: String(formData.get('jira_api_token') ?? '').trim(),
			jira_project_key: String(formData.get('jira_project_key') ?? '').trim(),
			linear_api_token: String(formData.get('linear_api_token') ?? '').trim(),
			linear_team_id: String(formData.get('linear_team_id') ?? '').trim()
		};

		const response = await fetch(`${API_BASE_URL}/v1/integrations/${provider}/connect`, {
			method: 'PUT',
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
			return fail(response.status, {
				action: 'connect',
				message: await response.text(),
				linear_api_token: payload.linear_api_token,
				linear_team_id: payload.linear_team_id
			});
		}

		const integrationPayload = (await response.json()) as {
			integration?: { config?: { team_id?: string } };
		};
		const connectedTeamID = String(integrationPayload.integration?.config?.team_id ?? '').trim();

		return {
			action: 'connect',
			success: true,
			message: 'Integration connected.',
			linear_api_token: payload.linear_api_token,
			linear_team_id: payload.linear_team_id || connectedTeamID
		};
	},

	runTest: async ({ cookies, fetch, request, params }) => {
		const session = cookies.get('session');
		if (!session) {
			throw redirect(302, '/auth');
		}

		const provider = params.provider.toLowerCase().trim();
		const formData = await request.formData();
		const connectPayload = {
			name: String(formData.get('name') ?? '').trim(),
			enabled: formData.get('enabled') === 'on',
			webhook_url: String(formData.get('webhook_url') ?? '').trim(),
			jira_base_url: String(formData.get('jira_base_url') ?? '').trim(),
			jira_email: String(formData.get('jira_email') ?? '').trim(),
			jira_api_token: String(formData.get('jira_api_token') ?? '').trim(),
			jira_project_key: String(formData.get('jira_project_key') ?? '').trim(),
			linear_api_token: String(formData.get('linear_api_token') ?? '').trim(),
			linear_team_id: String(formData.get('linear_team_id') ?? '').trim()
		};
		const headers = {
			Authorization: `Bearer ${session}`,
			'Content-Type': 'application/json'
		};

		const connectResponse = await fetch(`${API_BASE_URL}/v1/integrations/${provider}/connect`, {
			method: 'PUT',
			headers,
			body: JSON.stringify(connectPayload)
		});
		if (connectResponse.status === 401) {
			throw redirect(302, '/auth');
		}
		if (!connectResponse.ok) {
			return fail(connectResponse.status, {
				action: 'runTest',
				message: await connectResponse.text(),
				linear_api_token: connectPayload.linear_api_token,
				linear_team_id: connectPayload.linear_team_id
			});
		}

		const connectResult = (await connectResponse.json()) as {
			integration?: { config?: { team_id?: string } };
		};
		const effectiveTeamID =
			connectPayload.linear_team_id || String(connectResult.integration?.config?.team_id ?? '').trim();

		let response: Response;
		if (provider === 'slack' || provider === 'discord') {
			response = await fetch(`${API_BASE_URL}/v1/integrations/${provider}/messages`, {
				method: 'POST',
				headers,
				body: JSON.stringify({
					title: 'Arrakis integration test',
					severity: 'info',
					text: 'This is an automatic test message from your integration settings.'
				})
			});
		} else if (provider === 'github') {
			const repositoriesResponse = await fetch(
				`${API_BASE_URL}/v1/github/repositories?page=1&page_size=100`,
				{ headers: { Authorization: `Bearer ${session}` } }
			);
			if (repositoriesResponse.status === 401) {
				throw redirect(302, '/auth');
			}
			if (!repositoriesResponse.ok) {
				return fail(repositoriesResponse.status, {
					action: 'runTest',
					message: 'Failed to load connected repositories for GitHub test.'
				});
			}
			const repositoriesPayload = (await repositoriesResponse.json()) as {
				repositories?: Array<{ id: number; connected: boolean }>;
			};
			const firstConnected = (repositoriesPayload.repositories ?? []).find((repo) => repo.connected);
			if (!firstConnected) {
				return fail(400, {
					action: 'runTest',
					message: 'Connect at least one GitHub repository before running a test.'
				});
			}

			response = await fetch(`${API_BASE_URL}/v1/integrations/${provider}/issues`, {
				method: 'POST',
				headers,
				body: JSON.stringify({
					title: 'Arrakis integration test issue',
					description: 'This is an automatic test issue from your integration settings.',
					severity: 'low',
					repo_id: firstConnected.id,
					labels: ['arrakis-test']
				})
			});
		} else if (provider === 'jira' || provider === 'linear') {
			response = await fetch(`${API_BASE_URL}/v1/integrations/${provider}/issues`, {
				method: 'POST',
				headers,
				body: JSON.stringify({
					title: 'Arrakis integration test issue',
					description: 'This is an automatic test issue from your integration settings.',
					severity: 'low'
				})
			});
		} else {
			return fail(400, { action: 'runTest', message: 'Unsupported integration provider.' });
		}

		if (response.status === 401) {
			throw redirect(302, '/auth');
		}
		if (!response.ok) {
			return fail(response.status, {
				action: 'runTest',
				message: await response.text(),
				linear_api_token: connectPayload.linear_api_token,
				linear_team_id: effectiveTeamID
			});
		}

		return {
			action: 'runTest',
			success: true,
			message: 'Integration test sent successfully.',
			linear_api_token: connectPayload.linear_api_token,
			linear_team_id: effectiveTeamID
		};
	}
};
