<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import {
    api,
    ApiError,
    type SyncJob,
    type SyncPreviewResult,
    type SyncPreviewEntry
  } from '$lib/api';
  import { toasts } from '$lib/state/toasts.svelte';
  import { confirmDialog } from '$lib/state/confirm.svelte';
  import { t, tf, lang } from '$lib/i18n';
  import { flowKey } from '$lib/week';

  let jobs = $state<SyncJob[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let running = $state<string | null>(null);

  // Per-job quick preview state, keyed by job id.
  type PreviewState = {
    status: 'loading' | 'ok' | 'error';
    result?: SyncPreviewResult;
    message?: string;
  };
  let previews = $state<Record<string, PreviewState | undefined>>({});

  const PREVIEW_LIMIT = 8;

  async function togglePreview(job: SyncJob) {
    if (previews[job.id]) {
      previews[job.id] = undefined; // collapse
      return;
    }
    previews[job.id] = { status: 'loading' };
    try {
      const result = await api.sync.previewSaved(job.id);
      previews[job.id] = { status: 'ok', result };
    } catch (e) {
      previews[job.id] = {
        status: 'error',
        message: e instanceof ApiError ? e.message : t('preview_failed')
      };
    }
  }

  function capped(entries: SyncPreviewEntry[]): SyncPreviewEntry[] {
    return entries.slice(0, PREVIEW_LIMIT);
  }

  function fmtWhen(iso: string): string {
    if (!iso) return '';
    const d = new Date(iso);
    return isNaN(d.getTime()) ? iso : d.toLocaleDateString(lang);
  }

  async function load() {
    loading = true;
    error = null;
    try {
      jobs = await api.sync.list();
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        await goto('/login');
        return;
      }
      error = e instanceof Error ? e.message : t('load_failed');
    } finally {
      loading = false;
    }
  }
  onMount(load);

  async function run(job: SyncJob) {
    running = job.id;
    error = null;
    try {
      const updated = await api.sync.run(job.id);
      jobs = jobs.map((j) => (j.id === updated.id ? updated : j));
      const ok = updated.lastStatus.startsWith('ok');
      toasts.show(ok ? t('sync_complete') : `${updated.lastStatus}`, ok ? 'success' : 'error');
    } catch (e) {
      error = e instanceof Error ? e.message : t('save_failed');
    } finally {
      running = null;
    }
  }

  function formatLastRun(iso: string): string {
    if (!iso) return t('never');
    const d = new Date(iso);
    return isNaN(d.getTime()) ? iso : d.toLocaleString(lang);
  }

  // Explains the "+C ~U -D" counters, and for one-way jobs that the destination
  // is mirrored (deletions and edits there are undone on the next run). A
  // blocked job gets the instructions for its own way out instead.
  function statusHint(job: SyncJob): string {
    if (isBlocked(job)) return t('sync_blocked_hint');
    const hint = t('sync_status_hint');
    return job.direction === 'bidirectional' ? hint : `${hint} ${t('sync_mirror_hint')}`;
  }

  // The vanish guard stopped the run: it stays stopped until the state is reset.
  function isBlocked(job: SyncJob): boolean {
    return job.lastStatus.startsWith('blocked');
  }

  function statusClass(status: string): string {
    if (status.startsWith('ok')) return 'badge badge-ok';
    if (status.startsWith('error') || status.startsWith('config') || status.startsWith('blocked'))
      return 'badge badge-error';
    return 'badge';
  }

  async function resetState(job: SyncJob) {
    if (!(await confirmDialog.ask(tf('reset_state_confirm', { name: job.name }), t('reset_state'))))
      return;
    try {
      const updated = await api.sync.resetState(job.id);
      jobs = jobs.map((j) => (j.id === updated.id ? updated : j));
      toasts.show(t('sync_state_reset'));
    } catch (e) {
      error = e instanceof Error ? e.message : t('save_failed');
    }
  }

  async function remove(job: SyncJob) {
    if (!(await confirmDialog.ask(tf('delete_sync_confirm', { name: job.name }), t('delete'))))
      return;
    try {
      await api.sync.remove(job.id);
      jobs = jobs.filter((j) => j.id !== job.id);
      toasts.show(t('sync_job_deleted'));
    } catch (e) {
      error = e instanceof Error ? e.message : t('delete_failed');
    }
  }
</script>

<div class="head">
  <h1>{t('sync_title')}</h1>
  <a class="button" href="/sync/new">{t('new_sync_job')}</a>
</div>

{#if loading}
  <p class="muted">{t('loading')}</p>
{:else if error}
  <p class="error">{error}</p>
{:else if jobs.length === 0}
  <div class="card empty">
    <p>{t('no_sync_jobs')}</p>
    <a class="button" href="/sync/new">{t('create_first_sync')}</a>
  </div>
{:else}
  <div class="list">
    {#each jobs as job (job.id)}
      <div class="card job-card">
        <div class="job">
          <div class="info">
            <h2>
              {job.name}
              {#if !job.enabled}<span class="badge">{t('disabled')}</span>{/if}
            </h2>
            <div class="meta">
              <span class="badge">{job.kind}</span>
              <span class="badge">{job.direction}</span>
              <span class="badge">{Math.round(job.intervalSeconds / 60)}m</span>
            </div>
            <div class="run">
              <span class="last-run">{t('last_run')}: {formatLastRun(job.lastRunAt)}</span>
              {#if job.lastStatus}<span class={statusClass(job.lastStatus)} title={statusHint(job)}
                  >{job.lastStatus}</span
                >{/if}
            </div>
          </div>
          <div class="row-actions">
            <button class="button button-secondary" onclick={() => togglePreview(job)}>
              {previews[job.id] ? t('hide_preview') : t('preview')}
            </button>
            <button
              class="button button-secondary"
              onclick={() => run(job)}
              disabled={running === job.id}
            >
              {running === job.id ? t('running') : t('run_now')}
            </button>
            {#if isBlocked(job)}
              <button class="button button-secondary" onclick={() => resetState(job)}>
                {t('reset_state')}
              </button>
            {/if}
            <a class="button button-secondary" href={`/sync/${job.id}`}>{t('edit')}</a>
            <button class="button button-secondary danger" onclick={() => remove(job)}
              >{t('delete')}</button
            >
          </div>
        </div>

        {#if previews[job.id]}
          {@const p = previews[job.id]}
          <div class="quick-preview">
            {#if p?.status === 'loading'}
              <p class="muted">{t('previewing')}</p>
            {:else if p?.status === 'error'}
              <p class="error">{p.message}</p>
            {:else if p?.result}
              <div class="three">
                <div class="col">
                  <h3>{t('server_a')} <span class="badge">{p.result.a.length}</span></h3>
                  {#if p.result.a.length === 0}
                    <p class="muted">{t('no_entries')}</p>
                  {:else}
                    <ul>
                      {#each capped(p.result.a) as e, i (i)}
                        <li><span class="when">{fmtWhen(e.when)}</span> {e.title || e.uid}</li>
                      {/each}
                    </ul>
                    {#if p.result.a.length > PREVIEW_LIMIT}
                      <p class="muted">
                        {tf('preview_more', { n: p.result.a.length - PREVIEW_LIMIT })}
                      </p>
                    {/if}
                  {/if}
                </div>
                <div class="col">
                  <h3>{t('server_b')} <span class="badge">{p.result.b.length}</span></h3>
                  {#if p.result.b.length === 0}
                    <p class="muted">{t('no_entries')}</p>
                  {:else}
                    <ul>
                      {#each capped(p.result.b) as e, i (i)}
                        <li><span class="when">{fmtWhen(e.when)}</span> {e.title || e.uid}</li>
                      {/each}
                    </ul>
                    {#if p.result.b.length > PREVIEW_LIMIT}
                      <p class="muted">
                        {tf('preview_more', { n: p.result.b.length - PREVIEW_LIMIT })}
                      </p>
                    {/if}
                  {/if}
                </div>
                <div class="col result">
                  <h3>
                    {t('result')} ({t(flowKey(job.direction))})
                    <span class="badge badge-ok">{p.result.merged.length}</span>
                  </h3>
                  {#if p.result.merged.length === 0}
                    <p class="muted">{t('no_entries')}</p>
                  {:else}
                    <ul>
                      {#each capped(p.result.merged) as e, i (i)}
                        <li><span class="when">{fmtWhen(e.when)}</span> {e.title || e.uid}</li>
                      {/each}
                    </ul>
                    {#if p.result.merged.length > PREVIEW_LIMIT}
                      <p class="muted">
                        {tf('preview_more', { n: p.result.merged.length - PREVIEW_LIMIT })}
                      </p>
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
  .job-card {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .job {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }
  .quick-preview {
    border-top: 1px solid var(--separator);
    padding-top: var(--space-4);
  }
  .three {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: var(--space-4);
  }
  @media (max-width: 760px) {
    .three {
      grid-template-columns: 1fr;
    }
  }
  .three h3 {
    font-size: var(--text-sm);
    color: var(--text-secondary);
    margin: 0 0 var(--space-2);
  }
  .three ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  .three li {
    font-size: var(--text-sm);
    padding: var(--space-2) var(--space-3);
    background: var(--bg-base);
    border-radius: var(--radius-sm);
    border-left: 2px solid var(--separator);
  }
  .col.result li {
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
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .info h2 {
    font-size: var(--text-base);
    margin: 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .meta {
    display: flex;
    gap: var(--space-2);
  }
  .run {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  .last-run {
    color: var(--text-tertiary);
    font-size: var(--text-xs);
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
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-4);
  }
  .muted {
    color: var(--text-tertiary);
  }
  .error {
    color: var(--danger);
  }
</style>
