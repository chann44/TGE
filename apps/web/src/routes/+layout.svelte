<script lang="ts">
	import { page } from '$app/state';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import './layout.css';

	let { children } = $props();

	const navItems = [
		{ href: '/', label: 'Dashboard' },
		{ href: '/repos', label: 'Repositories' },
		{ href: '/policies', label: 'Policies' },
		{ href: '/dependencies', label: 'Dependencies' },
		{ href: '/scans', label: 'Scans' },
		{ href: '/settings', label: 'Settings' },
		{ href: '/system-health', label: 'System Health' }
	] as const;

	const isActive = (href: string) =>
		page.url.pathname === href || (href !== '/' && page.url.pathname.startsWith(`${href}/`));
</script>

<Sidebar.Provider>
	<Sidebar.Root variant="inset" collapsible="icon">
		<Sidebar.Header>
			<div class="rounded-md bg-sidebar-accent px-3 py-2">
				<p class="text-sm font-semibold">TEG Security</p>
				<p class="text-xs text-sidebar-foreground/70">Operations Console</p>
			</div>
		</Sidebar.Header>

		<Sidebar.Content>
			<Sidebar.Group>
				<Sidebar.GroupContent>
					<Sidebar.Menu>
						{#each navItems as item}
							<Sidebar.MenuItem>
								<Sidebar.MenuButton isActive={isActive(item.href)} tooltipContent={item.label}>
									{#snippet child({ props })}
										<a href={item.href} {...props}>
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
		<Sidebar.Rail />
	</Sidebar.Root>

	<Sidebar.Inset>
		<header
			class="sticky top-0 z-20 flex items-center gap-3 border-b border-border bg-background/80 px-4 py-3 backdrop-blur"
		>
			<Sidebar.Trigger class="md:hidden" />
			<p class="text-sm font-medium">{page.url.pathname}</p>
		</header>
		<div class="p-6">
			{@render children()}
		</div>
	</Sidebar.Inset>
</Sidebar.Provider>
