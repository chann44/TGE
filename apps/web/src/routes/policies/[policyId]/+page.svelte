<script lang="ts">
	import { page } from '$app/state';
</script>

<section class="mx-auto max-w-5xl space-y-4">
	<h1 class="text-3xl font-bold tracking-tight">Policy Details</h1>
	<p class="text-muted-foreground">
		Policy ID: <span class="font-medium text-foreground">{page.params.policyId}</span>
	</p>
</section>
