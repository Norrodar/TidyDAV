<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, ApiError, type Feed } from '$lib/api';
  import { toasts } from '$lib/state/toasts.svelte';
  import { confirmDialog } from '$lib/state/confirm.svelte';
  import { t, tf } from '$lib/i18n';

  let feeds = $state<Feed[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let copied = $state<string | null>(null);

  async function load() {
    loading = true;
    error = null;
    try {
      feeds = await api.feeds.list();
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        await goto('/login');
        return;
      }
      error = e instanceof Error ? e.message : 'Failed to load feeds';
    } finally {
      loading = false;
    }
  }

  onMount(load);

  async function remove(feed: Feed) {
    if (!(await confirmDialog.ask(tf('delete_calendar_confirm', { name: feed.name }), t('delete')))) return;
    try {
      await api.feeds.remove(feed.id);
      feeds = feeds.filter((f) => f.id !== feed.id);
      toasts.show(t('calendar_deleted'));
    } catch (e) {
      error = e instanceof Error ? e.message : t('delete_failed');
    }
  }

  async function copy(url: string) {
    if (!navigator.clipboard?.writeText) {
      error = 'Copy failed — select the URL manually.';
      return;
    }
    try {
      await navigator.clipboard.writeText(url);
      copied = url;
      setTimeout(() => (copied = null), 1500);
    } catch {
      error = 'Copy failed — select the URL manually.';
    }
  }
</script>

<div class="head">
  <h1>{t('calendars_title')}</h1>
  <a class="button" href="/feeds/new">{t('new_calendar')}</a>
</div>

{#if loading}
  <p class="muted">{t('loading')}</p>
{:else if error}
  <p class="error">{error}</p>
{:else if feeds.length === 0}
  <div class="card empty">
    <p>{t('no_calendars')}</p>
    <a class="button" href="/feeds/new">{t('create_first_calendar')}</a>
  </div>
{:else}
  <div class="list">
    {#each feeds as feed (feed.id)}
      <div class="card feed">
        <div class="info">
          <h2>{feed.name}</h2>
          <code class="url">{feed.icsUrl}</code>
          {#if feed.basicAuthEnabled}
            <p class="auth-hint">{t('basic_auth_hint')}</p>
          {/if}
        </div>
        <div class="meta">
          <span class="badge">{tf('source_count', { n: feed.sources.length })}</span>
          <span class="badge">{tf('rule_count', { n: feed.rules.length })}</span>
          {#if feed.basicAuthEnabled}<span class="badge">{t('basic_auth_badge')}</span>{/if}
        </div>
        <div class="row-actions">
          <button class="button button-secondary" onclick={() => copy(feed.icsUrl)}>
            {copied === feed.icsUrl ? t('copied') : t('copy_url')}
          </button>
          <a class="button button-secondary" href={`/feeds/${feed.id}`}>{t('edit')}</a>
          <button class="button button-secondary danger" onclick={() => remove(feed)}>{t('delete')}</button>
        </div>
      </div>
    {/each}
  </div>
{/if}

<style>
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-5);
  }
  h1 {
    font-size: var(--text-2xl);
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .feed {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }
  .info {
    flex: 1;
    min-width: 0;
  }
  .info h2 {
    font-size: var(--text-base);
    margin: 0 0 var(--space-1);
  }
  .url {
    color: var(--text-tertiary);
    font-size: var(--text-xs);
    word-break: break-all;
  }
  .auth-hint {
    margin: var(--space-1) 0 0;
    color: var(--text-tertiary);
    font-size: var(--text-xs);
  }
  .meta {
    display: flex;
    gap: var(--space-2);
  }
  .row-actions {
    display: flex;
    gap: var(--space-2);
  }
  .danger:hover {
    color: var(--danger);
    border-color: var(--danger);
  }
  .empty {
    align-items: flex-start;
    gap: var(--space-4);
    display: flex;
    flex-direction: column;
  }
  .muted {
    color: var(--text-tertiary);
  }
  .error {
    color: var(--danger);
  }
</style>
