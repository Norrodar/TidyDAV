<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api, ApiError, type SyncJob } from '$lib/api';
  import SyncJobEditor from '$lib/components/SyncJobEditor.svelte';
  import { t } from '$lib/i18n';

  let job = $state<SyncJob | null>(null);
  let error = $state<string | null>(null);

  onMount(async () => {
    const id = $page.params.id;
    if (!id) {
      error = 'missing sync job id';
      return;
    }
    try {
      job = await api.sync.get(id);
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        await goto('/login');
        return;
      }
      error = e instanceof Error ? e.message : 'Failed to load sync job';
    }
  });
</script>

<h1>{t('edit_sync_job')}</h1>
{#if error}
  <p class="error">{error}</p>
{:else if job}
  <SyncJobEditor {job} />
{:else}
  <p class="muted">{t('loading')}</p>
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
