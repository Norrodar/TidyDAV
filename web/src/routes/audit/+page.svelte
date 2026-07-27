<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, ApiError, type AuditEntry } from '$lib/api';
  import { t, lang } from '$lib/i18n';

  let entries = $state<AuditEntry[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);

  function fmtTime(iso: string): string {
    const d = new Date(iso);
    return isNaN(d.getTime()) ? iso : d.toLocaleString(lang);
  }

  onMount(async () => {
    try {
      entries = await api.audit.list();
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        await goto('/login');
        return;
      }
      if (e instanceof ApiError && e.status === 403) {
        error = t('audit_admin_only');
        return;
      }
      error = e instanceof Error ? e.message : t('load_failed');
    } finally {
      loading = false;
    }
  });
</script>

<h1>{t('audit_title')}</h1>

{#if loading}
  <p class="muted">{t('loading')}</p>
{:else if error}
  <p class="error">{error}</p>
{:else if entries.length === 0}
  <p class="muted">{t('audit_empty')}</p>
{:else}
  <div class="card">
    <table>
      <thead>
        <tr
          ><th>{t('audit_time')}</th><th>{t('audit_user')}</th><th>{t('audit_action')}</th><th
            >{t('audit_target')}</th
          ><th>{t('audit_detail')}</th></tr
        >
      </thead>
      <tbody>
        {#each entries as entry (entry.id)}
          <tr>
            <td class="mono">{fmtTime(entry.createdAt)}</td>
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
