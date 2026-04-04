import type { LayoutServerLoad } from './$types';

type MeResponse = {
	id: number;
	login: string;
	name: string;
	email: string;
	avatar_url: string;
};

const API_BASE_URL = 'http://localhost:8080';

export const load: LayoutServerLoad = async ({ cookies, fetch }) => {
	const session = cookies.get('session');
	if (!session) {
		return { user: null };
	}

	try {
		const response = await fetch(`${API_BASE_URL}/v1/me`, {
			headers: {
				Authorization: `Bearer ${session}`
			}
		});

		if (!response.ok) {
			return { user: null };
		}

		const me = (await response.json()) as MeResponse;
		return {
			user: {
				id: String(me.id),
				login: me.login,
				name: me.name || undefined,
				email: me.email || undefined,
				avatarUrl: me.avatar_url || undefined
			}
		};
	} catch {
		return { user: null };
	}
};
