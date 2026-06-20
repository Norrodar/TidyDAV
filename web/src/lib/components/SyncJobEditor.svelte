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
    type SyncConflict
  } from '$lib/api';

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

  let saving = $state(false);
  let error = $state<string | null>(null);

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
      enabled
    };
  }

  async function save() {
    saving = true;
    error = null;
    try {
      if (job) await api.sync.update(job.id, buildInput());
      else await api.sync.create(buildInput());
      await goto('/sync');
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Save failed';
    } finally {
      saving = false;
    }
  }
</script>

<form onsubmit={(e) => { e.preventDefault(); save(); }}>
  <section class="card">
    <label class="field">
      <span>Name</span>
      <input class="input" bind:value={name} placeholder="Calendar sync" required />
    </label>
    <div class="row">
      <label class="field grow">
        <span>Type</span>
        <select class="input" bind:value={kind}>
          <option value="caldav">CalDAV (calendars)</option>
          <option value="carddav">CardDAV (contacts)</option>
        </select>
      </label>
      <label class="field grow">
        <span>Direction</span>
        <select class="input" bind:value={direction}>
          <option value="a-to-b">A → B</option>
          <option value="b-to-a">B → A</option>
          <option value="bidirectional">Bidirectional</option>
        </select>
      </label>
      <label class="field grow">
        <span>Conflict (bidirectional)</span>
        <select class="input" bind:value={conflict}>
          <option value="newest-wins">Newest wins</option>
          <option value="source-wins">A (source) wins</option>
        </select>
      </label>
    </div>
  </section>

  <div class="endpoints">
    <section class="card">
      <h2>Server A</h2>
      <label class="field">
        <span>Collection URL</span>
        <input class="input" bind:value={aUrl} placeholder="https://a.example.com/dav/cal/" required />
      </label>
      <div class="row">
        <label class="field grow">
          <span>Username</span>
          <input class="input" bind:value={aUsername} />
        </label>
        <label class="field grow">
          <span>Password</span>
          <input class="input" type="password" bind:value={aPassword} placeholder={initial?.aPasswordSet ? 'unchanged' : ''} />
        </label>
      </div>
    </section>

    <section class="card">
      <h2>Server B</h2>
      <label class="field">
        <span>Collection URL</span>
        <input class="input" bind:value={bUrl} placeholder="https://b.example.com/dav/cal/" required />
      </label>
      <div class="row">
        <label class="field grow">
          <span>Username</span>
          <input class="input" bind:value={bUsername} />
        </label>
        <label class="field grow">
          <span>Password</span>
          <input class="input" type="password" bind:value={bPassword} placeholder={initial?.bPasswordSet ? 'unchanged' : ''} />
        </label>
      </div>
    </section>
  </div>

  <section class="card">
    <div class="row">
      <label class="field">
        <span>Interval (minutes)</span>
        <input class="input narrow" type="number" min="1" bind:value={intervalMinutes} />
      </label>
      <label class="check">
        <input type="checkbox" bind:checked={enabled} /> Enabled
      </label>
    </div>
  </section>

  {#if error}<p class="error">{error}</p>{/if}

  <div class="actions">
    <button type="submit" class="button" disabled={saving}>
      {saving ? 'Saving…' : job ? 'Save changes' : 'Create job'}
    </button>
    <a class="button button-secondary" href="/sync">Cancel</a>
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
    grid-template-columns: 1fr 1fr;
    gap: var(--space-4);
  }
  h2 {
    font-size: var(--text-lg);
    margin: 0;
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
  .actions {
    display: flex;
    gap: var(--space-3);
    align-items: center;
  }
  .error {
    color: var(--danger);
    font-size: var(--text-sm);
  }
</style>
