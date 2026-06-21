<script lang="ts">
  import SyncJobEditor from '$lib/components/SyncJobEditor.svelte';
  import { t } from '$lib/i18n';
</script>

<h1>{t('new_sync_heading')}</h1>
<SyncJobEditor />

<style>
  h1 {
    font-size: var(--text-2xl);
    margin-bottom: var(--space-5);
  }
</style>
