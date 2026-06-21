<script lang="ts">
  import { untrack } from 'svelte';
  import { goto } from '$app/navigation';
  import {
    api,
    ApiError,
    type SyncJob,
    type SyncJobInput,
    type SyncKind,
    type SyncDirection,
    type SyncConflict,
    type SyncPreviewResult
  } from '$lib/api';
  import { toasts } from '$lib/state/toasts.svelte';
  import { t, tf, lang } from '$lib/i18n';
  import { weekStartISO, nextDirection, flowKey } from '$lib/week';

  let { job }: { job?: SyncJob } = $props();
  const initial = untrack(() => job);

  let name = $state(initial?.name ?? '');
  let kind = $state<SyncKind>(initial?.kind ?? 'caldav');
  let direction = $state<SyncDirection>(initial?.direction ?? 'a-to-b');
  let conflict = $state<SyncConflict>(initial?.conflict ?? 'newest-wins');
  let aUrl = $state(initial?.aUrl ?? '');
  let aUsername = $state(initial?.aUsername ?? '');
  let aPassword = $state('');
  let bUrl = $state(initial?.bUrl ?? '');
  let bUsername = $state(initial?.bUsername ?? '');
  let bPassword = $state('');
  let intervalMinutes = $state(initial ? Math.max(1, Math.round(initial.intervalSeconds / 60)) : 15);
  let enabled = $state(initial?.enabled ?? true);
  let rangeEnabled = $state(!!(initial?.windowStart || initial?.windowEnd));
  let windowStart = $state(initial?.windowStart ?? '');
  let windowEnd = $state(initial?.windowEnd ?? '');

  let saving = $state(false);
  let error = $state<string | null>(null);

  // Preview
  let preview = $state<SyncPreviewResult | null>(null);
  let previewing = $state(false);
  let weekOffset = $state(0);
  let panelOpen = $state(true);

  function cycleDirection() {
    direction = nextDirection(direction);
  }

  function buildInput(): SyncJobInput {
    return {
      name,
      kind,
      direction,
      conflict,
      aUrl: aUrl.trim(),
      aUsername: aUsername || undefined,
      aPassword: aPassword || undefined,
      bUrl: bUrl.trim(),
      bUsername: bUsername || undefined,
      bPassword: bPassword || undefined,
      intervalSeconds: Math.max(60, Math.round(intervalMinutes) * 60),
      enabled,
      windowStart: rangeEnabled && kind === 'caldav' ? windowStart : '',
      windowEnd: rangeEnabled && kind === 'caldav' ? windowEnd : ''
    };
  }

  async function save() {
    saving = true;
    error = null;
    try {
      if (job) await api.sync.update(job.id, buildInput());
      else await api.sync.create(buildInput());
      toasts.show(job ? t('sync_job_saved') : t('sync_job_created'));
      await goto('/sync');
    } catch (e) {
      error = e instanceof ApiError ? e.message : t('save_failed');
    } finally {
      saving = false;
    }
  }

  const weekLabel = $derived(new Date(weekStartISO(weekOffset)).toLocaleDateString(lang));

  async function runPreview() {
    previewing = true;
    error = null;
    panelOpen = true;
    try {
      preview = await api.sync.preview(
        {
          kind,
          direction,
          aUrl: aUrl.trim(),
          aUsername: aUsername || undefined,
          aPassword: aPassword || undefined,
          bUrl: bUrl.trim(),
          bUsername: bUsername || undefined,
          bPassword: bPassword || undefined,
          weekStart: kind === 'caldav' ? weekStartISO(weekOffset) : undefined
        },
        job?.id
      );
    } catch (e) {
      error = e instanceof ApiError ? e.message : t('preview_failed');
    } finally {
      previewing = false;
    }
  }

  function fmtWhen(iso: string): string {
    if (!iso) return '';
    const d = new Date(iso);
    return isNaN(d.getTime()) ? iso : d.toLocaleDateString(lang);
  }
</script>

<form onsubmit={(e) => { e.preventDefault(); save(); }}>
  <section class="card">
    <label class="field">
      <span>{t('name')}</span>
      <input class="input" bind:value={name} placeholder={t('sync_name_placeholder')} required />
    </label>
    <div class="row wrap">
      <label class="field grow">
        <span>{t('type')}</span>
        <select class="input" bind:value={kind}>
          <option value="caldav">{t('caldav_label')}</option>
          <option value="carddav">{t('carddav_label')}</option>
        </select>
      </label>
      {#if direction === 'bidirectional'}
        <label class="field grow">
          <span>{t('conflict')}</span>
          <select class="input" bind:value={conflict}>
            <option value="newest-wins">{t('newest_wins')}</option>
            <option value="source-wins">{t('server_a_wins')}</option>
          </select>
        </label>
      {/if}
    </div>
  </section>

  <div class="endpoints">
    <section class="card">
      <h2>{t('server_a')}</h2>
      <label class="field">
        <span>{t('collection_url')}</span>
        <input class="input" bind:value={aUrl} placeholder="https://a.example.com/dav/cal/" required />
      </label>
      <div class="row wrap">
        <label class="field grow">
          <span>{t('username')}</span>
          <input class="input" bind:value={aUsername} autocomplete="off" />
        </label>
        <label class="field grow">
          <span>{t('password')}</span>
          <input class="input" type="password" bind:value={aPassword} autocomplete="new-password" placeholder={initial?.aPasswordSet ? t('unchanged') : ''} />
        </label>
      </div>
    </section>

    <div class="flow">
      <button type="button" class="flow-btn" onclick={cycleDirection} title={t('flow_hint')} aria-label={t(flowKey(direction))}>
        {#if direction === 'a-to-b'}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="4" y1="12" x2="20" y2="12" /><polyline points="14 6 20 12 14 18" />
          </svg>
        {:else if direction === 'b-to-a'}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="20" y1="12" x2="4" y2="12" /><polyline points="10 6 4 12 10 18" />
          </svg>
        {:else}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="7 4 3 8 7 12" /><line x1="3" y1="8" x2="21" y2="8" />
            <polyline points="17 12 21 16 17 20" /><line x1="21" y1="16" x2="3" y2="16" />
          </svg>
        {/if}
      </button>
      <span class="flow-label">{t(flowKey(direction))}</span>
    </div>

    <section class="card">
      <h2>{t('server_b')}</h2>
      <label class="field">
        <span>{t('collection_url')}</span>
        <input class="input" bind:value={bUrl} placeholder="https://b.example.com/dav/cal/" required />
      </label>
      <div class="row wrap">
        <label class="field grow">
          <span>{t('username')}</span>
          <input class="input" bind:value={bUsername} autocomplete="off" />
        </label>
        <label class="field grow">
          <span>{t('password')}</span>
          <input class="input" type="password" bind:value={bPassword} autocomplete="new-password" placeholder={initial?.bPasswordSet ? t('unchanged') : ''} />
        </label>
      </div>
    </section>
  </div>

  {#if preview}
    <section class="card preview">
      <div class="panel-head">
        <h2>{t('preview')}</h2>
        <button type="button" class="linklike" onclick={() => (panelOpen = !panelOpen)}>
          {panelOpen ? t('hide_preview') : t('show_preview')}
        </button>
      </div>
      {#if panelOpen}
        {#if kind === 'caldav'}
          <div class="week-nav">
            <button type="button" class="button button-secondary button-sm" onclick={() => { weekOffset--; runPreview(); }}>‹ {t('prev_week')}</button>
            <span class="week-label">{tf('this_week', { date: weekLabel })}</span>
            <button type="button" class="button button-secondary button-sm" onclick={() => { weekOffset++; runPreview(); }}>{t('next_week')} ›</button>
          </div>
        {/if}
        <div class="three">
          <div class="col">
            <h3>{t('server_a')} <span class="badge">{preview.a.length}</span></h3>
            <ul>
              {#each preview.a as e, i (i)}
                <li><span class="when">{fmtWhen(e.when)}</span> {e.title || e.uid}</li>
              {/each}
            </ul>
          </div>
          <div class="col">
            <h3>{t('server_b')} <span class="badge">{preview.b.length}</span></h3>
            <ul>
              {#each preview.b as e, i (i)}
                <li><span class="when">{fmtWhen(e.when)}</span> {e.title || e.uid}</li>
              {/each}
            </ul>
          </div>
          <div class="col result">
            <h3>{t('result')} ({t(flowKey(direction))}) <span class="badge badge-ok">{preview.merged.length}</span></h3>
            <ul>
              {#each preview.merged as e, i (i)}
                <li><span class="when">{fmtWhen(e.when)}</span> {e.title || e.uid}</li>
              {/each}
            </ul>
          </div>
        </div>
      {/if}
    </section>
  {/if}

  <section class="card">
    <label class="check">
      <input type="checkbox" bind:checked={enabled} /> {t('enable_recurring')}
    </label>
    <div class="row interval" class:disabled={!enabled}>
      <label class="field">
        <span>{t('interval_minutes')}</span>
        <input class="input narrow" type="number" min="1" bind:value={intervalMinutes} disabled={!enabled} />
      </label>
      <span class="status">{enabled ? tf('status_every', { n: intervalMinutes }) : t('status_one_time')}</span>
    </div>
  </section>

  {#if kind === 'caldav'}
    <section class="card">
      <label class="check">
        <input type="checkbox" bind:checked={rangeEnabled} /> {t('limit_date_range')}
      </label>
      {#if rangeEnabled}
        <div class="row wrap">
          <label class="field">
            <span>{t('date_from')}</span>
            <input class="input" type="date" bind:value={windowStart} />
          </label>
          <label class="field">
            <span>{t('date_to')}</span>
            <input class="input" type="date" bind:value={windowEnd} />
          </label>
        </div>
      {/if}
    </section>
  {/if}

  {#if error}<p class="error">{error}</p>{/if}

  <div class="actions">
    <button type="button" class="button button-secondary" onclick={runPreview} disabled={previewing}>
      {previewing ? t('previewing') : t('load_preview_week')}
    </button>
    <button type="submit" class="button" disabled={saving}>
      {saving ? t('saving') : job ? t('save_changes') : t('create_job')}
    </button>
    <a class="button button-secondary" href="/sync">{t('cancel')}</a>
  </div>
</form>

<style>
  form {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .endpoints {
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    gap: var(--space-4);
    align-items: center;
  }
  .endpoints :global(.card) {
    border-top: 2px solid var(--accent);
  }
  @media (max-width: 760px) {
    .endpoints {
      grid-template-columns: 1fr;
    }
  }
  h2 {
    font-size: var(--text-lg);
    margin: 0;
  }
  h3 {
    font-size: var(--text-sm);
    color: var(--text-secondary);
    margin: 0 0 var(--space-2);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-secondary);
  }
  .row {
    display: flex;
    gap: var(--space-3);
    align-items: flex-end;
  }
  .row.wrap {
    flex-wrap: wrap;
  }
  .grow {
    flex: 1;
    min-width: 160px;
  }
  .narrow {
    width: 120px;
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-secondary);
  }
  .interval {
    align-items: center;
  }
  .interval.disabled {
    opacity: 0.55;
  }
  .status {
    font-size: var(--text-sm);
    color: var(--text-tertiary);
  }
  .flow {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
  }
  @media (max-width: 760px) {
    .flow {
      flex-direction: row;
      justify-content: center;
    }
  }
  .flow-btn {
    width: 48px;
    height: 48px;
    border-radius: var(--radius-md);
    border: 1px solid var(--separator);
    background: var(--bg-elevated);
    color: var(--accent);
    cursor: pointer;
    display: grid;
    place-items: center;
    transition: all var(--dur-fast) var(--ease);
  }
  .flow-btn:hover {
    border-color: var(--accent);
    transform: scale(1.05);
  }
  .flow-btn svg {
    width: 24px;
    height: 24px;
  }
  .flow-label {
    font-size: var(--text-xs);
    color: var(--text-tertiary);
    text-align: center;
  }
  .actions {
    display: flex;
    gap: var(--space-3);
    align-items: center;
    flex-wrap: wrap;
  }
  .error {
    color: var(--danger);
    font-size: var(--text-sm);
  }
  .panel-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .linklike {
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    color: var(--accent);
    font-size: var(--text-sm);
  }
  .button-sm {
    padding: var(--space-1) var(--space-3);
    font-size: var(--text-sm);
  }
  .week-nav {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }
  .week-label {
    font-size: var(--text-xs);
    color: var(--text-secondary);
    flex: 1;
    text-align: center;
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
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  .col.result {
    padding: var(--space-3);
    margin: calc(-1 * var(--space-3));
    border-radius: var(--radius-md);
    background: rgba(48, 209, 88, 0.05);
  }
  li {
    font-size: var(--text-sm);
    padding: var(--space-2) var(--space-3);
    background: var(--bg-base);
    border-radius: var(--radius-sm);
    border-left: 2px solid var(--separator);
    transition: border-color var(--dur-fast) var(--ease);
  }
  li:hover {
    border-left-color: var(--accent);
  }
  .result li {
    border-left-color: var(--success);
  }
  .when {
    color: var(--text-tertiary);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    margin-right: var(--space-2);
  }
</style>
