<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api, ApiError, type Feed } from '$lib/api';
  import FeedEditor from '$lib/components/FeedEditor.svelte';

  let feed = $state<Feed | null>(null);
  let error = $state<string | null>(null);

  onMount(async () => {
    const id = $page.params.id;
    if (!id) {
      error = 'missing feed id';
      return;
    }
    try {
      feed = await api.feeds.get(id);
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        await goto('/login');
        return;
      }
      error = e instanceof Error ? e.message : 'Failed to load feed';
    }
  });
</script>

<h1>Edit feed</h1>
{#if error}
  <p class="error">{error}</p>
{:else if feed}
  <FeedEditor {feed} />
{:else}
  <p class="muted">Loading…</p>
{/if}

<style>
  h1 {
    font-size: var(--text-2xl);
    margin-bottom: var(--space-5);
  }
  .error {
    color: var(--danger);
  }
  .muted {
    color: var(--text-tertiary);
  }
</style>
