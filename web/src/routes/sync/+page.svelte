<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, ApiError, type SyncJob } from '$lib/api';
  import { toasts } from '$lib/state/toasts.svelte';
  import { confirmDialog } from '$lib/state/confirm.svelte';
  import { t, tf, lang } from '$lib/i18n';

  let jobs = $state<SyncJob[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let running = $state<string | null>(null);

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
      error = e instanceof Error ? e.message : 'Failed to load sync jobs';
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

  function statusClass(status: string): string {
    if (status.startsWith('ok')) return 'badge badge-ok';
    if (status.startsWith('error') || status.startsWith('config')) return 'badge badge-error';
    return 'badge';
  }

  async function remove(job: SyncJob) {
    if (!(await confirmDialog.ask(tf('delete_sync_confirm', { name: job.name }), t('delete')))) return;
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
      <div class="card job">
        <div class="info">
          <h2>{job.name} {#if !job.enabled}<span class="badge">{t('disabled')}</span>{/if}</h2>
          <div class="meta">
            <span class="badge">{job.kind}</span>
            <span class="badge">{job.direction}</span>
            <span class="badge">{Math.round(job.intervalSeconds / 60)}m</span>
          </div>
          <div class="run">
            <span class="last-run">{t('last_run')}: {formatLastRun(job.lastRunAt)}</span>
            {#if job.lastStatus}<span class={statusClass(job.lastStatus)}>{job.lastStatus}</span>{/if}
          </div>
        </div>
        <div class="row-actions">
          <button class="button button-secondary" onclick={() => run(job)} disabled={running === job.id}>
            {running === job.id ? t('running') : t('run_now')}
          </button>
          <a class="button button-secondary" href={`/sync/${job.id}`}>{t('edit')}</a>
          <button class="button button-secondary danger" onclick={() => remove(job)}>{t('delete')}</button>
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
  .job {
    display: flex;
    align-items: center;
    gap: var(--space-4);
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
