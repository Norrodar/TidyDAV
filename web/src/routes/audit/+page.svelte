<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, ApiError, type AuditEntry } from '$lib/api';

  let entries = $state<AuditEntry[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  onMount(async () => {
    try {
      entries = await api.audit.list();
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        await goto('/login');
        return;
      }
      if (e instanceof ApiError && e.status === 403) {
        error = 'Admin access required.';
        return;
      }
      error = e instanceof Error ? e.message : 'Failed to load audit log';
    } finally {
      loading = false;
    }
  });
</script>

<h1>Audit log</h1>

{#if loading}
  <p class="muted">Loading…</p>
{:else if error}
  <p class="error">{error}</p>
{:else if entries.length === 0}
  <p class="muted">No entries yet.</p>
{:else}
  <div class="card">
    <table>
      <thead>
        <tr><th>Time</th><th>User</th><th>Action</th><th>Target</th><th>Detail</th></tr>
      </thead>
      <tbody>
        {#each entries as entry (entry.id)}
          <tr>
            <td class="mono">{entry.createdAt}</td>
            <td>{entry.userEmail || '—'}</td>
            <td>{entry.action}</td>
            <td class="mono">{entry.target || '—'}</td>
            <td>{entry.detail || '—'}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<style>
  h1 {
    font-size: var(--text-2xl);
    margin-bottom: var(--space-5);
  }
  .card {
    overflow-x: auto;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--text-sm);
  }
  th {
    text-align: left;
    color: var(--text-secondary);
    font-weight: var(--weight-medium);
    padding: var(--space-2) var(--space-3);
    border-bottom: 1px solid var(--separator);
  }
  td {
    padding: var(--space-2) var(--space-3);
    border-bottom: 1px solid var(--separator);
    color: var(--text-primary);
  }
  tr:last-child td {
    border-bottom: none;
  }
  .mono {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-secondary);
  }
  .muted {
    color: var(--text-tertiary);
  }
  .error {
    color: var(--danger);
  }
</style>
