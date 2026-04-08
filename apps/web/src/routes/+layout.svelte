<script lang="ts">
	import { page } from '$app/state';
	import {
		Home01Icon,
		PackageIcon,
		PolicyIcon,
		RepositoryIcon,
		ScanIcon,
		Shield01Icon,
		ServerStack01Icon,
		Settings01Icon
	} from '@hugeicons/core-free-icons';
	import { HugeiconsIcon } from '@hugeicons/svelte';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import '@fontsource-variable/inter';
	import './layout.css';

	let { children, data } = $props();

	const navItems = [
		{ href: '/', label: 'Dashboard', icon: Home01Icon },
		{ href: '/repos', label: 'Repositories', icon: RepositoryIcon },
		{ href: '/findings', label: 'Findings', icon: Shield01Icon },
		{ href: '/policies', label: 'Policies', icon: PolicyIcon },
		{ href: '/dependencies', label: 'Dependencies', icon: PackageIcon },
		{ href: '/scans', label: 'Scans', icon: ScanIcon },
		{ href: '/integrations', label: 'Integrations', icon: Settings01Icon },
		{ href: '/settings', label: 'Settings', icon: Settings01Icon },
		{ href: '/system-health', label: 'System Health', icon: ServerStack01Icon }
	] as const;

	const isActive = (href: string) =>
		page.url.pathname === href || (href !== '/' && page.url.pathname.startsWith(`${href}/`));
</script>

<Sidebar.Provider style="--sidebar-width: 12.5rem; --sidebar-width-icon: 2.4rem;">
	<Sidebar.Root variant="inset" collapsible="icon">
		<Sidebar.Header>
			<div class="rounded-md border border-sidebar-border bg-sidebar-accent px-3 py-2">
				<p class="text-sm font-bold tracking-tight text-primary">Arrakis</p>
			</div>
		</Sidebar.Header>

		<Sidebar.Content>
			<Sidebar.Group>
				<Sidebar.GroupContent>
					<Sidebar.Menu>
						{#each navItems as item}
							<Sidebar.MenuItem>
								<Sidebar.MenuButton
									size="sm"
									isActive={isActive(item.href)}
									tooltipContent={item.label}
									class="rounded-md text-[11px] data-[active=true]:bg-primary data-[active=true]:text-primary-foreground"
								>
									{#snippet child({ props })}
										<a href={item.href} {...props}>
											<HugeiconsIcon icon={item.icon} strokeWidth={1.8} />
											<span>{item.label}</span>
										</a>
									{/snippet}
								</Sidebar.MenuButton>
							</Sidebar.MenuItem>
						{/each}
					</Sidebar.Menu>
				</Sidebar.GroupContent>
			</Sidebar.Group>
		</Sidebar.Content>
		<Sidebar.Footer>
			{#if data?.user}
				<form method="POST" action="/auth/logout" class="mb-2">
					<button
						type="submit"
						class="w-full rounded-md border border-sidebar-border bg-sidebar-accent px-2 py-1.5 text-left text-[11px] text-sidebar-foreground hover:bg-sidebar-accent/70"
					>
						Logout
					</button>
				</form>
			{/if}
			<div class="rounded-md border border-sidebar-border bg-sidebar-accent px-2 py-1.5">
				<p class="text-[10px] text-primary">v0.0.1 - self-hosted</p>
				<div class="mt-1 flex items-center gap-1.5">
					<span class="inline-block h-1.5 w-1.5 rounded-full bg-emerald-400"></span>
					<span class="text-[10px] text-sidebar-foreground/80">All systems nominal</span>
				</div>
			</div>
		</Sidebar.Footer>
		<Sidebar.Rail />
	</Sidebar.Root>

	<Sidebar.Inset>
		<header
			class="sticky top-0 z-20 flex items-center gap-2 border-b border-border bg-background/80 px-3 py-2 backdrop-blur"
		>
			<Sidebar.Trigger class="md:hidden" />
			<p class="text-xs font-medium">{page.url.pathname}</p>
			<div class="ml-auto flex items-center gap-2">
				{#if data?.user}
					{#if data.user.avatarUrl}
						<img
							src={data.user.avatarUrl}
							alt={data.user.login}
							class="h-7 w-7 rounded-full border border-border object-cover"
						/>
					{:else}
						<div
							class="flex h-7 w-7 items-center justify-center rounded-full border border-border bg-primary text-[10px] font-semibold text-primary-foreground uppercase"
						>
							{data.user.login.slice(0, 1)}
						</div>
					{/if}
					<span class="text-xs text-muted-foreground">{data.user.login}</span>
				{:else}
					<a
						href="/auth"
						class="rounded-md border border-primary bg-primary px-3 py-1 text-xs text-primary-foreground"
					>
						Login
					</a>
				{/if}
			</div>
		</header>
		<div class="p-3 md:p-4">
			{@render children()}
		</div>
	</Sidebar.Inset>
</Sidebar.Provider>
