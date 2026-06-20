<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, ApiError, type SyncJob } from '$lib/api';

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
    } catch (e) {
      error = e instanceof Error ? e.message : 'Run failed';
    } finally {
      running = null;
    }
  }

  async function remove(job: SyncJob) {
    if (!confirm(`Delete sync job “${job.name}”?`)) return;
    try {
      await api.sync.remove(job.id);
      jobs = jobs.filter((j) => j.id !== job.id);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Delete failed';
    }
  }
</script>

<div class="head">
  <h1>DAV sync</h1>
  <a class="button" href="/sync/new">New sync job</a>
</div>

{#if loading}
  <p class="muted">Loading…</p>
{:else if error}
  <p class="error">{error}</p>
{:else if jobs.length === 0}
  <div class="card empty">
    <p>No sync jobs yet.</p>
    <a class="button" href="/sync/new">Create your first sync job</a>
  </div>
{:else}
  <div class="list">
    {#each jobs as job (job.id)}
      <div class="card job">
        <div class="info">
          <h2>{job.name} {#if !job.enabled}<span class="badge">disabled</span>{/if}</h2>
          <div class="meta">
            <span class="badge">{job.kind}</span>
            <span class="badge">{job.direction}</span>
            <span class="badge">{Math.round(job.intervalSeconds / 60)}m</span>
          </div>
          {#if job.lastStatus}<code class="status">{job.lastStatus}</code>{/if}
        </div>
        <div class="row-actions">
          <button class="button button-secondary" onclick={() => run(job)} disabled={running === job.id}>
            {running === job.id ? 'Running…' : 'Run now'}
          </button>
          <a class="button button-secondary" href={`/sync/${job.id}`}>Edit</a>
          <button class="button button-secondary danger" onclick={() => remove(job)}>Delete</button>
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
  .status {
    color: var(--text-tertiary);
    font-size: var(--text-xs);
    word-break: break-all;
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
