<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, ApiError, type Feed, type PreviewResult, type EventSummary } from '$lib/api';
  import { toasts } from '$lib/state/toasts.svelte';
  import { confirmDialog } from '$lib/state/confirm.svelte';
  import { t, tf, lang } from '$lib/i18n';

  let feeds = $state<Feed[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let copied = $state<string | null>(null);

  // Per-feed quick preview state, keyed by feed id.
  type PreviewState = { status: 'loading' | 'ok' | 'error'; result?: PreviewResult; message?: string };
  let previews = $state<Record<string, PreviewState | undefined>>({});

  const PREVIEW_LIMIT = 8;

  async function togglePreview(feed: Feed) {
    if (previews[feed.id]) {
      previews[feed.id] = undefined; // collapse
      return;
    }
    previews[feed.id] = { status: 'loading' };
    try {
      const result = await api.feeds.previewSaved(feed.id);
      previews[feed.id] = { status: 'ok', result };
    } catch (e) {
      previews[feed.id] = {
        status: 'error',
        message: e instanceof ApiError ? e.message : t('preview_failed')
      };
    }
  }

  // Show what the calendar will display next: upcoming events first, capped.
  function upcoming(events: EventSummary[]): EventSummary[] {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const future = events.filter((e) => {
      if (!e.start) return true;
      const d = new Date(e.start);
      return isNaN(d.getTime()) || d >= today;
    });
    return (future.length ? future : events).slice(0, PREVIEW_LIMIT);
  }

  function fmtWhen(iso: string): string {
    if (!iso) return '—';
    const d = new Date(iso);
    return isNaN(d.getTime()) ? iso : d.toLocaleDateString(lang);
  }

  function fmtDateTime(iso: string): string {
    const d = new Date(iso);
    return isNaN(d.getTime()) ? iso : d.toLocaleString(lang);
  }

  // Source health: ok (fetched within 24h), stale (older) or never fetched.
  type Health = 'ok' | 'stale' | 'never';
  function sourceHealth(lastFetchedAt?: string): Health {
    if (!lastFetchedAt) return 'never';
    const d = new Date(lastFetchedAt);
    if (isNaN(d.getTime())) return 'never';
    return Date.now() - d.getTime() > 24 * 3600 * 1000 ? 'stale' : 'ok';
  }
  function healthTitle(lastFetchedAt?: string): string {
    const h = sourceHealth(lastFetchedAt);
    if (h === 'never') return t('health_never');
    const time = fmtDateTime(lastFetchedAt!);
    return h === 'ok' ? tf('health_ok', { time }) : tf('health_stale', { time });
  }
  function worstHealth(feed: Feed): Health {
    let worst: Health = 'ok';
    for (const s of feed.sources) {
      const h = sourceHealth(s.lastFetchedAt);
      if (h === 'stale') return 'stale';
      if (h === 'never') worst = 'never';
    }
    return feed.sources.length ? worst : 'never';
  }

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
      <div class="card feed-card">
        <div class="feed">
          <div class="info">
            <h2>
              <span class="health {worstHealth(feed)}" title={feed.sources.map((s) => healthTitle(s.lastFetchedAt)).join('\n')}></span>
              {feed.name}
            </h2>
            <code class="url">{feed.icsUrl}</code>
            {#if feed.basicAuthEnabled}
              <p class="auth-hint">{t('basic_auth_hint')}</p>
            {/if}
            <p class="stats">
              {feed.lastServedAt
                ? tf('client_last_fetch', { time: fmtDateTime(feed.lastServedAt), n: feed.serveCount })
                : t('client_never')}
            </p>
          </div>
          <div class="meta">
            <span class="badge">{tf('source_count', { n: feed.sources.length })}</span>
            <span class="badge">{tf('rule_count', { n: feed.rules.length })}</span>
            {#if feed.basicAuthEnabled}<span class="badge">{t('basic_auth_badge')}</span>{/if}
          </div>
          <div class="row-actions">
            <button class="button button-secondary" onclick={() => togglePreview(feed)}>
              {previews[feed.id] ? t('hide_preview') : t('preview')}
            </button>
            <button class="button button-secondary" onclick={() => copy(feed.icsUrl)}>
              {copied === feed.icsUrl ? t('copied') : t('copy_url')}
            </button>
            <a class="button button-secondary" href={`/feeds/${feed.id}`}>{t('edit')}</a>
            <button class="button button-secondary danger" onclick={() => remove(feed)}>{t('delete')}</button>
          </div>
        </div>

        {#if previews[feed.id]}
          {@const p = previews[feed.id]}
          <div class="quick-preview">
            {#if p?.status === 'loading'}
              <p class="muted">{t('previewing')}</p>
            {:else if p?.status === 'error'}
              <p class="error">{p.message}</p>
            {:else if p?.result}
              <div class="diff">
                <div class="diff-col">
                  <h3>{t('original')} <span class="badge">{p.result.original.length}</span></h3>
                  {#if p.result.original.length === 0}
                    <p class="muted">{t('no_entries')}</p>
                  {:else}
                    <ul>
                      {#each upcoming(p.result.original) as e, i (i)}
                        <li><span class="when">{fmtWhen(e.start)}</span> {e.summary}</li>
                      {/each}
                    </ul>
                    {#if p.result.original.length > PREVIEW_LIMIT}
                      <p class="muted">{tf('preview_more', { n: p.result.original.length - PREVIEW_LIMIT })}</p>
                    {/if}
                  {/if}
                </div>
                <div class="diff-col transformed">
                  <h3>{t('transformed')} <span class="badge badge-ok">{p.result.transformed.length}</span></h3>
                  {#if p.result.transformed.length === 0}
                    <p class="muted">{t('no_entries')}</p>
                  {:else}
                    <ul>
                      {#each upcoming(p.result.transformed) as e, i (i)}
                        <li><span class="when">{fmtWhen(e.start)}</span> {e.summary}</li>
                      {/each}
                    </ul>
                    {#if p.result.transformed.length > PREVIEW_LIMIT}
                      <p class="muted">{tf('preview_more', { n: p.result.transformed.length - PREVIEW_LIMIT })}</p>
                    {/if}
                  {/if}
                </div>
              </div>
            {/if}
          </div>
        {/if}
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
  .feed-card {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .feed {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }
  .quick-preview {
    border-top: 1px solid var(--separator);
    padding-top: var(--space-4);
  }
  .diff {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-4);
  }
  @media (max-width: 640px) {
    .diff {
      grid-template-columns: 1fr;
    }
  }
  .diff h3 {
    font-size: var(--text-sm);
    color: var(--text-secondary);
    margin: 0 0 var(--space-2);
  }
  .diff ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  .diff li {
    font-size: var(--text-sm);
    padding: var(--space-2) var(--space-3);
    background: var(--bg-base);
    border-radius: var(--radius-sm);
    border-left: 2px solid var(--separator);
  }
  .diff-col.transformed li {
    border-left-color: var(--success);
  }
  .when {
    color: var(--text-tertiary);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    margin-right: var(--space-2);
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
  .stats {
    margin: var(--space-1) 0 0;
    color: var(--text-tertiary);
    font-size: var(--text-xs);
  }
  .info h2 {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .health {
    flex-shrink: 0;
    width: 9px;
    height: 9px;
    border-radius: var(--radius-full);
    background: var(--success);
    cursor: default;
  }
  .health.stale {
    background: var(--warning);
  }
  .health.never {
    background: var(--text-tertiary);
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
