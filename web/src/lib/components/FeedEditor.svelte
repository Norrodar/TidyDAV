<script lang="ts">
  import { untrack } from 'svelte';
  import { goto } from '$app/navigation';
  import {
    api,
    ApiError,
    type Feed,
    type FeedInput,
    type RuleConfig,
    type RuleType,
    type PreviewResult
  } from '$lib/api';

  let { feed }: { feed?: Feed } = $props();

  // The editor is mounted with a fixed feed; capture its values once.
  const initial = untrack(() => feed);

  let name = $state(initial?.name ?? '');
  let ttlSeconds = $state(initial?.ttlSeconds ?? 900);
  let sources = $state(
    initial && initial.sources.length
      ? initial.sources.map((s) => ({ url: s.url, username: s.username ?? '', password: '' }))
      : [{ url: '', username: '', password: '' }]
  );
  let rules = $state<RuleConfig[]>(initial ? initial.rules.map((r) => ({ ...r })) : []);
  let basicAuthUser = $state(initial?.basicAuthUser ?? '');
  let basicAuthPassword = $state('');

  let notifyWebhook = $state(initial?.notifications.webhookUrl ?? '');
  let notifyNtfyServer = $state(initial?.notifications.ntfyServer ?? '');
  let notifyNtfyTopic = $state(initial?.notifications.ntfyTopic ?? '');
  let notifyGotifyServer = $state(initial?.notifications.gotifyServer ?? '');
  let notifyGotifyToken = $state('');
  let notifyTriggers = $state<string[]>(initial?.notifications.triggers ?? []);

  let saving = $state(false);
  let previewing = $state(false);
  let error = $state<string | null>(null);
  let preview = $state<PreviewResult | null>(null);

  const ruleTypes: RuleType[] = ['filter', 'dedup', 'rename', 'strip', 'timezone', 'expire'];

  function defaultRule(type: RuleType): RuleConfig {
    switch (type) {
      case 'filter':
        return { type, filterMode: 'blacklist', matchMode: 'substring', pattern: '' };
      case 'rename':
        return { type, field: 'SUMMARY', matchMode: 'substring', pattern: '', replacement: '' };
      case 'dedup':
        return { type, keyFields: [] };
      case 'strip':
        return { type, fields: [] };
      case 'timezone':
        return { type, target: 'UTC', defaultTz: '' };
      case 'expire':
        return { type, days: 90 };
    }
  }

  function addSource() {
    sources = [...sources, { url: '', username: '', password: '' }];
  }
  function removeSource(i: number) {
    sources = sources.filter((_, idx) => idx !== i);
  }
  function addRule() {
    rules = [...rules, defaultRule('filter')];
  }
  function removeRule(i: number) {
    rules = rules.filter((_, idx) => idx !== i);
  }
  function changeRuleType(i: number, type: RuleType) {
    rules[i] = defaultRule(type);
  }

  function toggleTrigger(type: string) {
    notifyTriggers = notifyTriggers.includes(type)
      ? notifyTriggers.filter((t) => t !== type)
      : [...notifyTriggers, type];
  }

  function csv(arr?: string[]): string {
    return (arr ?? []).join(', ');
  }
  function setCsv(rule: RuleConfig, key: 'fields' | 'keyFields', value: string) {
    rule[key] = value
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);
  }

  function buildInput(): FeedInput {
    return {
      name,
      ttlSeconds,
      sources: sources
        .filter((s) => s.url.trim() !== '')
        .map((s) => ({
          url: s.url.trim(),
          username: s.username || undefined,
          password: s.password || undefined
        })),
      rules,
      basicAuthUser,
      basicAuthPassword: basicAuthPassword || undefined,
      notifications: {
        webhookUrl: notifyWebhook || undefined,
        ntfyServer: notifyNtfyServer || undefined,
        ntfyTopic: notifyNtfyTopic || undefined,
        gotifyServer: notifyGotifyServer || undefined,
        gotifyToken: notifyGotifyToken || undefined,
        triggers: notifyTriggers
      }
    };
  }

  async function save() {
    saving = true;
    error = null;
    try {
      if (feed) await api.feeds.update(feed.id, buildInput());
      else await api.feeds.create(buildInput());
      await goto('/feeds');
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Save failed';
    } finally {
      saving = false;
    }
  }

  async function runPreview() {
    previewing = true;
    error = null;
    try {
      preview = await api.feeds.preview(buildInput(), feed?.id);
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Preview failed';
    } finally {
      previewing = false;
    }
  }
</script>

<form onsubmit={(e) => { e.preventDefault(); save(); }}>
  <section class="card">
    <label class="field">
      <span>Name</span>
      <input class="input" bind:value={name} placeholder="Waste collection" required />
    </label>
  </section>

  <section class="card">
    <div class="section-head">
      <h2>Sources</h2>
      <button type="button" class="button button-secondary" onclick={addSource}>Add source</button>
    </div>
    {#each sources as source, i (i)}
      <div class="row">
        <input class="input grow" bind:value={source.url} placeholder="https://example.com/feed.ics" />
        <input class="input" bind:value={source.username} placeholder="user (optional)" />
        <input
          class="input"
          type="password"
          bind:value={source.password}
          autocomplete="new-password"
          placeholder="password (optional)"
        />
        <button type="button" class="icon" onclick={() => removeSource(i)} aria-label="Remove">×</button>
      </div>
    {/each}
  </section>

  <section class="card">
    <div class="section-head">
      <h2>Rules</h2>
      <button type="button" class="button button-secondary" onclick={addRule}>Add rule</button>
    </div>
    {#if rules.length === 0}
      <p class="muted">No rules — the merged feed is served as-is.</p>
    {/if}
    {#each rules as rule, i (i)}
      <div class="rule">
        <div class="rule-head">
          <select
            class="input"
            value={rule.type}
            onchange={(e) => changeRuleType(i, e.currentTarget.value as RuleType)}
          >
            {#each ruleTypes as rt}
              <option value={rt}>{rt}</option>
            {/each}
          </select>
          <button type="button" class="icon" onclick={() => removeRule(i)} aria-label="Remove">×</button>
        </div>

        {#if rule.type === 'filter'}
          <div class="rule-fields">
            <select class="input" bind:value={rule.filterMode}>
              <option value="blacklist">blacklist (remove matches)</option>
              <option value="whitelist">whitelist (keep matches)</option>
            </select>
            <select class="input" bind:value={rule.matchMode}>
              <option value="substring">substring</option>
              <option value="regex">regex</option>
            </select>
            <input class="input grow" bind:value={rule.pattern} placeholder="pattern" />
            <input
              class="input grow"
              value={csv(rule.fields)}
              oninput={(e) => setCsv(rule, 'fields', e.currentTarget.value)}
              placeholder="fields (default: SUMMARY, DESCRIPTION, LOCATION, CATEGORIES)"
            />
          </div>
        {:else if rule.type === 'rename'}
          <div class="rule-fields">
            <select class="input" bind:value={rule.field}>
              <option value="SUMMARY">SUMMARY</option>
              <option value="DESCRIPTION">DESCRIPTION</option>
              <option value="LOCATION">LOCATION</option>
            </select>
            <select class="input" bind:value={rule.matchMode}>
              <option value="substring">substring</option>
              <option value="regex">regex</option>
            </select>
            <input class="input grow" bind:value={rule.pattern} placeholder="pattern" />
            <input class="input grow" bind:value={rule.replacement} placeholder="replacement ($1 in regex)" />
          </div>
        {:else if rule.type === 'dedup'}
          <div class="rule-fields">
            <input
              class="input grow"
              value={csv(rule.keyFields)}
              oninput={(e) => setCsv(rule, 'keyFields', e.currentTarget.value)}
              placeholder="key fields (default: SUMMARY, DATE)"
            />
          </div>
        {:else if rule.type === 'strip'}
          <div class="rule-fields">
            <input
              class="input grow"
              value={csv(rule.fields)}
              oninput={(e) => setCsv(rule, 'fields', e.currentTarget.value)}
              placeholder="fields to remove, e.g. DESCRIPTION, LOCATION, URL"
            />
          </div>
        {:else if rule.type === 'timezone'}
          <div class="rule-fields">
            <input class="input grow" bind:value={rule.target} placeholder="target, e.g. Europe/Berlin" />
            <input class="input grow" bind:value={rule.defaultTz} placeholder="default for floating times (optional)" />
          </div>
        {:else if rule.type === 'expire'}
          <div class="rule-fields">
            <label class="inline">
              Drop events older than
              <input class="input narrow" type="number" min="1" bind:value={rule.days} /> days
            </label>
          </div>
        {/if}
      </div>
    {/each}
  </section>

  <section class="card">
    <h2>Advanced</h2>
    <label class="field">
      <span>Cache TTL (seconds)</span>
      <input class="input narrow" type="number" min="0" bind:value={ttlSeconds} />
    </label>
    <div class="row">
      <label class="field grow">
        <span>Basic auth user (optional)</span>
        <input class="input" bind:value={basicAuthUser} placeholder="leave empty to disable" />
      </label>
      <label class="field grow">
        <span>Basic auth password</span>
        <input
          class="input"
          type="password"
          bind:value={basicAuthPassword}
          autocomplete="new-password"
          placeholder={feed?.basicAuthEnabled ? 'unchanged' : ''}
        />
      </label>
    </div>
  </section>

  <section class="card">
    <h2>Notifications</h2>
    <p class="muted">
      Fire a notification when matching rules trigger. Checked on a schedule (not on every
      calendar refresh), and each matched event notifies only once.
    </p>
    <div class="triggers">
      <span>Trigger on:</span>
      <label>
        <input
          type="checkbox"
          checked={notifyTriggers.includes('filter')}
          onchange={() => toggleTrigger('filter')}
        /> filter
      </label>
      <label>
        <input
          type="checkbox"
          checked={notifyTriggers.includes('rename')}
          onchange={() => toggleTrigger('rename')}
        /> rename
      </label>
    </div>
    <label class="field">
      <span>Webhook URL</span>
      <input class="input" bind:value={notifyWebhook} placeholder="https://… (HTTP POST JSON)" />
    </label>
    <div class="row">
      <label class="field grow">
        <span>ntfy server</span>
        <input class="input" bind:value={notifyNtfyServer} placeholder="https://ntfy.sh" />
      </label>
      <label class="field grow">
        <span>ntfy topic</span>
        <input class="input" bind:value={notifyNtfyTopic} />
      </label>
    </div>
    <div class="row">
      <label class="field grow">
        <span>Gotify server</span>
        <input class="input" bind:value={notifyGotifyServer} placeholder="https://gotify.example.com" />
      </label>
      <label class="field grow">
        <span>Gotify token</span>
        <input
          class="input"
          type="password"
          bind:value={notifyGotifyToken}
          autocomplete="new-password"
          placeholder={initial?.notifications.gotifyTokenSet ? 'unchanged' : ''}
        />
      </label>
    </div>
  </section>

  {#if error}<p class="error">{error}</p>{/if}

  <div class="actions">
    <button type="button" class="button button-secondary" onclick={runPreview} disabled={previewing}>
      {previewing ? 'Previewing…' : 'Preview'}
    </button>
    <button type="submit" class="button" disabled={saving}>
      {saving ? 'Saving…' : feed ? 'Save changes' : 'Create feed'}
    </button>
    <a class="button button-secondary" href="/feeds">Cancel</a>
  </div>
</form>

{#if preview}
  <section class="card preview">
    <h2>Preview</h2>
    <div class="diff">
      <div>
        <h3>Original <span class="badge">{preview.original.length}</span></h3>
        <ul>
          {#each preview.original as e (e.uid + e.start)}
            <li><span class="when">{e.start || '—'}</span> {e.summary}</li>
          {/each}
        </ul>
      </div>
      <div>
        <h3>Transformed <span class="badge badge-ok">{preview.transformed.length}</span></h3>
        <ul>
          {#each preview.transformed as e (e.uid + e.start)}
            <li><span class="when">{e.start || '—'}</span> {e.summary}</li>
          {/each}
        </ul>
      </div>
    </div>
  </section>
{/if}

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
  .section-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
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
    gap: var(--space-2);
    align-items: center;
  }
  .grow {
    flex: 1;
    min-width: 0;
  }
  .narrow {
    width: 120px;
  }
  .rule {
    border: 1px solid var(--separator);
    border-radius: var(--radius-md);
    padding: var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .rule-head {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }
  .rule-head select {
    width: 160px;
  }
  .rule-fields {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .inline {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--text-secondary);
    font-size: var(--text-sm);
  }
  .icon {
    width: 36px;
    height: 36px;
    border-radius: var(--radius-md);
    border: 1px solid var(--separator);
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: var(--text-lg);
    line-height: 1;
  }
  .icon:hover {
    color: var(--danger);
    border-color: var(--danger);
  }
  .actions {
    display: flex;
    gap: var(--space-3);
    align-items: center;
  }
  .muted {
    color: var(--text-tertiary);
    font-size: var(--text-sm);
    margin: 0;
  }
  .triggers {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    font-size: var(--text-sm);
    color: var(--text-secondary);
  }
  .triggers label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .error {
    color: var(--danger);
    font-size: var(--text-sm);
  }
  .preview {
    margin-top: var(--space-4);
  }
  .diff {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-5);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  li {
    font-size: var(--text-sm);
    padding: var(--space-2) var(--space-3);
    background: var(--bg-base);
    border-radius: var(--radius-sm);
  }
  .when {
    color: var(--text-tertiary);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    margin-right: var(--space-2);
  }
</style>
