<script lang="ts">
  import FeedEditor from '$lib/components/FeedEditor.svelte';
  import { t } from '$lib/i18n';
</script>

<h1>{t('new_calendar_heading')}</h1>
<FeedEditor />

<style>
  h1 {
    font-size: var(--text-2xl);
    margin-bottom: var(--space-5);
  }
</style>
