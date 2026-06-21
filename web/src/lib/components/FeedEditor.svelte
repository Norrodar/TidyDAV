<script lang="ts">
  import { untrack, onMount } from 'svelte';
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
  import { toasts } from '$lib/state/toasts.svelte';
  import { t, tf, lang } from '$lib/i18n';
  import { weekStartDate, inWeek } from '$lib/week';

  let { feed }: { feed?: Feed } = $props();
  const initial = untrack(() => feed);

  type SourceStatus = 'idle' | 'checking' | 'ok' | 'error';
  type SourceRow = {
    url: string;
    username: string;
    password: string;
    useAuth: boolean;
    status: SourceStatus;
    statusMsg: string;
  };

  let name = $state(initial?.name ?? '');
  let ttlSeconds = $state(initial?.ttlSeconds ?? 900);
  let sources = $state<SourceRow[]>(
    initial && initial.sources.length
      ? initial.sources.map((s) => ({
          url: s.url,
          username: s.username ?? '',
          password: '',
          useAuth: !!(s.username || s.hasPassword),
          status: 'idle' as SourceStatus,
          statusMsg: ''
        }))
      : [{ url: '', username: '', password: '', useAuth: false, status: 'idle' as SourceStatus, statusMsg: '' }]
  );
  let rules = $state<RuleConfig[]>(initial ? initial.rules.map((r) => ({ ...r })) : []);
  let basicAuthUser = $state(initial?.basicAuthUser ?? '');
  let basicAuthPassword = $state('');
  let advancedEnabled = $state(
    !!(initial && (initial.ttlSeconds !== 900 || initial.basicAuthUser || initial.basicAuthEnabled))
  );

  let notifyWebhook = $state(initial?.notifications.webhookUrl ?? '');
  let notifyNtfyServer = $state(initial?.notifications.ntfyServer ?? '');
  let notifyNtfyTopic = $state(initial?.notifications.ntfyTopic ?? '');
  let notifyGotifyServer = $state(initial?.notifications.gotifyServer ?? '');
  let notifyGotifyToken = $state('');
  let notifyTriggers = $state<string[]>(initial?.notifications.triggers ?? []);
  let webhookEnabled = $state(!!initial?.notifications.webhookUrl);
  let ntfyEnabled = $state(!!(initial?.notifications.ntfyServer || initial?.notifications.ntfyTopic));
  let gotifyEnabled = $state(
    !!(initial?.notifications.gotifyServer || initial?.notifications.gotifyTokenSet)
  );

  let saving = $state(false);
  let previewing = $state(false);
  let error = $state<string | null>(null);
  let preview = $state<PreviewResult | null>(null);
  let weekOffset = $state(0);
  let panelOpen = $state(true);

  const ruleTypes: RuleType[] = ['filter', 'dedup', 'rename', 'strip', 'timezone', 'expire'];

  // Known fields offered as click chips for each rule kind.
  const dedupChips = ['SUMMARY', 'DATE', 'LOCATION', 'DESCRIPTION', 'CATEGORIES'];
  const filterChips = ['SUMMARY', 'DESCRIPTION', 'LOCATION', 'CATEGORIES'];
  const stripChips = ['DESCRIPTION', 'LOCATION', 'CATEGORIES', 'URL'];

  function ruleLabel(type: RuleType): string {
    return t(`rule_${type}`);
  }
  function ruleHelp(type: RuleType): string {
    return t(`help_${type}`);
  }
  function fieldLabel(f: string): string {
    switch (f.toUpperCase()) {
      case 'SUMMARY':
        return t('field_summary');
      case 'DESCRIPTION':
        return t('field_description');
      case 'LOCATION':
        return t('field_location');
      case 'CATEGORIES':
        return t('field_categories');
      case 'DATE':
      case 'DTSTART':
        return t('field_dtstart');
      default:
        return f;
    }
  }

  function isEnabled(rule: RuleConfig): boolean {
    return rule.enabled !== false;
  }
  function toggleEnabled(rule: RuleConfig) {
    rule.enabled = rule.enabled === false ? true : false;
  }

  function defaultRule(type: RuleType): RuleConfig {
    switch (type) {
      case 'filter':
        return { type, filterMode: 'blacklist', matchMode: 'substring', pattern: '', fields: [] };
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
    sources = [...sources, { url: '', username: '', password: '', useAuth: false, status: 'idle', statusMsg: '' }];
  }
  function removeSource(i: number) {
    sources = sources.filter((_, idx) => idx !== i);
  }

  // ── Per-source validation (debounced) ────────────────────────────────────────
  const checkTimers: Record<number, ReturnType<typeof setTimeout>> = {};
  function scheduleCheck(i: number) {
    clearTimeout(checkTimers[i]);
    if (!sources[i].url.trim()) {
      sources[i].status = 'idle';
      sources[i].statusMsg = '';
      return;
    }
    checkTimers[i] = setTimeout(() => runSourceCheck(i), 600);
  }
  async function runSourceCheck(i: number) {
    const s = sources[i];
    if (!s || !s.url.trim()) return;
    s.status = 'checking';
    try {
      const res = await api.feeds.checkSource({
        url: s.url.trim(),
        username: s.useAuth ? s.username || undefined : undefined,
        password: s.useAuth ? s.password || undefined : undefined,
        id: feed?.id
      });
      if (sources[i] !== s) return; // row moved/removed mid-flight
      s.status = res.ok ? 'ok' : 'error';
      s.statusMsg = res.ok ? tf('source_valid', { n: res.events }) : (res.error ?? 'invalid');
    } catch (e) {
      s.status = 'error';
      s.statusMsg = e instanceof ApiError ? e.message : 'check failed';
    }
  }

  onMount(() => {
    sources.forEach((s, i) => {
      if (s.url.trim()) runSourceCheck(i);
    });
  });
  function addRule() {
    rules = [...rules, defaultRule('filter')];
  }
  function removeRule(i: number) {
    rules = rules.filter((_, idx) => idx !== i);
  }
  function changeRuleType(i: number, type: RuleType) {
    rules[i] = defaultRule(type);
  }

  // ── Drag & drop reordering ───────────────────────────────────────────────────
  // The stored order is the execution order: the pipeline applies rules in array
  // sequence, so reordering here changes how the calendar is processed.
  let dragIndex = $state<number | null>(null);
  let dragOverIndex = $state<number | null>(null);

  function onRuleDragStart(e: DragEvent, i: number) {
    dragIndex = i;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', String(i));
    }
  }
  function onRuleDragOver(e: DragEvent, i: number) {
    if (dragIndex === null) return;
    e.preventDefault();
    dragOverIndex = i;
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
  }
  function onRuleDrop(e: DragEvent, i: number) {
    e.preventDefault();
    if (dragIndex !== null && dragIndex !== i) {
      const arr = rules.slice();
      const [moved] = arr.splice(dragIndex, 1);
      arr.splice(i, 0, moved);
      rules = arr;
    }
    dragIndex = null;
    dragOverIndex = null;
  }
  function onRuleDragEnd() {
    dragIndex = null;
    dragOverIndex = null;
  }
  // Keyboard-accessible reordering alternative to drag & drop.
  function moveRule(i: number, delta: number) {
    const j = i + delta;
    if (j < 0 || j >= rules.length) return;
    const arr = rules.slice();
    [arr[i], arr[j]] = [arr[j], arr[i]];
    rules = arr;
  }

  function toggleTrigger(type: string) {
    notifyTriggers = notifyTriggers.includes(type)
      ? notifyTriggers.filter((x) => x !== type)
      : [...notifyTriggers, type];
  }

  // Field chip / custom helpers operate on the stored fields/keyFields arrays.
  function hasField(arr: string[] | undefined, f: string): boolean {
    return (arr ?? []).some((x) => x.toUpperCase() === f.toUpperCase());
  }
  function toggleField(rule: RuleConfig, key: 'fields' | 'keyFields', f: string) {
    const cur = (rule[key] ?? []).slice();
    const idx = cur.findIndex((x) => x.toUpperCase() === f.toUpperCase());
    if (idx >= 0) cur.splice(idx, 1);
    else cur.push(f);
    rule[key] = cur;
  }
  function customFields(arr: string[] | undefined, known: string[]): string {
    const up = known.map((k) => k.toUpperCase());
    return (arr ?? []).filter((f) => !up.includes(f.toUpperCase())).join(', ');
  }
  function setCustomFields(rule: RuleConfig, key: 'fields' | 'keyFields', known: string[], value: string) {
    const up = known.map((k) => k.toUpperCase());
    const kept = (rule[key] ?? []).filter((f) => up.includes(f.toUpperCase()));
    const customs = value.split(',').map((s) => s.trim()).filter(Boolean);
    rule[key] = [...kept, ...customs];
  }

  function buildInput(): FeedInput {
    return {
      name,
      ttlSeconds: advancedEnabled ? ttlSeconds : 900,
      sources: sources
        .filter((s) => s.url.trim() !== '')
        .map((s) => ({
          url: s.url.trim(),
          username: s.useAuth ? s.username || undefined : undefined,
          password: s.useAuth ? s.password || undefined : undefined
        })),
      rules,
      basicAuthUser: advancedEnabled ? basicAuthUser : '',
      basicAuthPassword: advancedEnabled ? basicAuthPassword || undefined : undefined,
      notifications: {
        webhookUrl: webhookEnabled ? notifyWebhook || undefined : '',
        ntfyServer: ntfyEnabled ? notifyNtfyServer || undefined : '',
        ntfyTopic: ntfyEnabled ? notifyNtfyTopic || undefined : '',
        gotifyServer: gotifyEnabled ? notifyGotifyServer || undefined : '',
        gotifyToken: gotifyEnabled ? notifyGotifyToken || undefined : undefined,
        triggers: notifyTriggers
      }
    };
  }

  // Returns a message for the first enabled rule with an invalid regex, else null.
  function regexError(): string | null {
    for (let i = 0; i < rules.length; i++) {
      const r = rules[i];
      if (!isEnabled(r)) continue;
      if ((r.type === 'filter' || r.type === 'rename') && r.matchMode === 'regex' && r.pattern) {
        try {
          new RegExp(r.pattern);
        } catch {
          return `${ruleLabel(r.type)} #${i + 1}: invalid regular expression.`;
        }
      }
    }
    return null;
  }

  async function save() {
    const re = regexError();
    if (re) {
      error = re;
      return;
    }
    saving = true;
    error = null;
    try {
      if (feed) await api.feeds.update(feed.id, buildInput());
      else await api.feeds.create(buildInput());
      toasts.show(feed ? t('calendar_saved') : t('calendar_created'));
      await goto('/feeds');
    } catch (e) {
      error = e instanceof ApiError ? e.message : t('save_failed');
    } finally {
      saving = false;
    }
  }

  async function runPreview() {
    const re = regexError();
    if (re) {
      error = re;
      return;
    }
    previewing = true;
    error = null;
    panelOpen = true;
    try {
      preview = await api.feeds.preview(buildInput(), feed?.id);
    } catch (e) {
      error = e instanceof ApiError ? e.message : t('preview_failed');
    } finally {
      previewing = false;
    }
  }

  // ── Week windowing for the preview ───────────────────────────────────────────
  const currentWeekStart = $derived(weekStartDate(weekOffset));
  function fmtWhen(iso: string): string {
    if (!iso) return '—';
    const d = new Date(iso);
    return isNaN(d.getTime()) ? iso : d.toLocaleDateString(lang);
  }
  const weekOriginal = $derived(
    (preview?.original ?? []).filter((e) => inWeek(e.start, currentWeekStart))
  );
  const weekTransformed = $derived(
    (preview?.transformed ?? []).filter((e) => inWeek(e.start, currentWeekStart))
  );
</script>

<div class="editor-layout">
  <form onsubmit={(e) => { e.preventDefault(); save(); }}>
    <section class="card">
      <label class="field">
        <span>{t('name')}</span>
        <input class="input" bind:value={name} placeholder={t('name_placeholder')} required />
      </label>
    </section>

    <section class="card">
      <div class="section-head">
        <h2>{t('sources')}</h2>
        <button type="button" class="button button-secondary" onclick={addSource}>{t('add_source')}</button>
      </div>
      {#each sources as source, i (i)}
        <div class="source">
          <div class="row">
            <div class="src-input grow">
              <input
                class="input"
                bind:value={source.url}
                oninput={() => scheduleCheck(i)}
                placeholder={t('source_url_placeholder')}
              />
              {#if source.status !== 'idle'}
                <span
                  class="src-status {source.status}"
                  title={source.status === 'checking' ? t('source_checking') : source.statusMsg}
                >
                  {#if source.status === 'checking'}
                    <span class="spinner"></span>
                  {:else if source.status === 'ok'}
                    ✓
                  {:else}
                    ✕
                  {/if}
                </span>
              {/if}
            </div>
            <button type="button" class="icon" onclick={() => removeSource(i)} aria-label={t('remove')}>×</button>
          </div>
          <label class="check">
            <input type="checkbox" bind:checked={source.useAuth} onchange={() => scheduleCheck(i)} /> {t('use_credentials')}
          </label>
          <div class="row creds" class:disabled={!source.useAuth}>
            <input
              class="input grow"
              bind:value={source.username}
              oninput={() => scheduleCheck(i)}
              disabled={!source.useAuth}
              autocomplete="off"
              placeholder={t('username')}
            />
            <input
              class="input grow"
              type="password"
              bind:value={source.password}
              oninput={() => scheduleCheck(i)}
              disabled={!source.useAuth}
              autocomplete="new-password"
              placeholder={t('password')}
            />
          </div>
        </div>
      {/each}
    </section>

    <section class="card">
      <div class="section-head">
        <h2>{t('rules')}</h2>
        <button type="button" class="button button-secondary" onclick={addRule}>{t('add_rule')}</button>
      </div>
      <p class="muted">{rules.length === 0 ? t('no_rules') : t('rules_apply_order')}</p>

      {#each rules as rule, i (i)}
        <div
          class="rule"
          class:off={!isEnabled(rule)}
          class:drag-over={dragOverIndex === i && dragIndex !== i}
          class:dragging={dragIndex === i}
          ondragover={(e) => onRuleDragOver(e, i)}
          ondrop={(e) => onRuleDrop(e, i)}
          role="group"
        >
          <div class="rule-head">
            <span
              class="drag-handle"
              draggable="true"
              ondragstart={(e) => onRuleDragStart(e, i)}
              ondragend={onRuleDragEnd}
              title={t('reorder_rule')}
              aria-label={t('reorder_rule')}
              role="button"
              tabindex="-1"
            >
              <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                <circle cx="9" cy="6" r="1.6" /><circle cx="15" cy="6" r="1.6" />
                <circle cx="9" cy="12" r="1.6" /><circle cx="15" cy="12" r="1.6" />
                <circle cx="9" cy="18" r="1.6" /><circle cx="15" cy="18" r="1.6" />
              </svg>
            </span>
            <span class="move-btns">
              <button type="button" class="move" onclick={() => moveRule(i, -1)} disabled={i === 0} aria-label={t('move_up')} title={t('move_up')}>▲</button>
              <button type="button" class="move" onclick={() => moveRule(i, 1)} disabled={i === rules.length - 1} aria-label={t('move_down')} title={t('move_down')}>▼</button>
            </span>
            <span class="rule-num">{i + 1}</span>
            <select
              class="input"
              value={rule.type}
              onchange={(e) => changeRuleType(i, e.currentTarget.value as RuleType)}
            >
              {#each ruleTypes as rt}
                <option value={rt}>{ruleLabel(rt)}</option>
              {/each}
            </select>
            <label class="toggle">
              <input type="checkbox" checked={isEnabled(rule)} onchange={() => toggleEnabled(rule)} />
              {t('rule_enabled')}
            </label>
            <button type="button" class="icon" onclick={() => removeRule(i)} aria-label={t('remove')}>×</button>
          </div>
          <p class="rule-desc">{ruleHelp(rule.type)}</p>

          {#if rule.type === 'filter'}
            <div class="rule-fields">
              <select class="input" bind:value={rule.filterMode}>
                <option value="blacklist">{t('filter_blacklist')}</option>
                <option value="whitelist">{t('filter_whitelist')}</option>
              </select>
              <select class="input" bind:value={rule.matchMode}>
                <option value="substring">{t('match_substring')}</option>
                <option value="regex">{t('match_regex')}</option>
              </select>
              <input class="input grow" bind:value={rule.pattern} placeholder={t('pattern')} />
            </div>
            <div class="chips-label">{t('fields_to_match')}</div>
            <div class="chips">
              {#each filterChips as f}
                <button
                  type="button"
                  class="chip"
                  class:on={hasField(rule.fields, f)}
                  onclick={() => toggleField(rule, 'fields', f)}
                >{fieldLabel(f)}</button>
              {/each}
            </div>
            <input
              class="input"
              value={customFields(rule.fields, filterChips)}
              oninput={(e) => setCustomFields(rule, 'fields', filterChips, e.currentTarget.value)}
              placeholder={t('custom_fields')}
            />
          {:else if rule.type === 'rename'}
            <div class="rule-fields">
              <select class="input" bind:value={rule.field}>
                <option value="SUMMARY">{t('field_summary')}</option>
                <option value="DESCRIPTION">{t('field_description')}</option>
                <option value="LOCATION">{t('field_location')}</option>
              </select>
              <select class="input" bind:value={rule.matchMode}>
                <option value="substring">{t('match_substring')}</option>
                <option value="regex">{t('match_regex')}</option>
              </select>
              <input class="input grow" bind:value={rule.pattern} placeholder={t('pattern')} />
              <input class="input grow" bind:value={rule.replacement} placeholder={t('replacement')} />
            </div>
          {:else if rule.type === 'dedup'}
            <div class="chips-label">{t('key_fields')}</div>
            <div class="chips">
              {#each dedupChips as f}
                <button
                  type="button"
                  class="chip"
                  class:on={hasField(rule.keyFields, f)}
                  onclick={() => toggleField(rule, 'keyFields', f)}
                >{fieldLabel(f)}</button>
              {/each}
            </div>
            <input
              class="input"
              value={customFields(rule.keyFields, dedupChips)}
              oninput={(e) => setCustomFields(rule, 'keyFields', dedupChips, e.currentTarget.value)}
              placeholder={t('custom_fields')}
            />
          {:else if rule.type === 'strip'}
            <div class="chips-label">{t('fields_to_strip')}</div>
            <div class="chips">
              {#each stripChips as f}
                <button
                  type="button"
                  class="chip"
                  class:on={hasField(rule.fields, f)}
                  onclick={() => toggleField(rule, 'fields', f)}
                >{fieldLabel(f)}</button>
              {/each}
            </div>
            <input
              class="input"
              value={customFields(rule.fields, stripChips)}
              oninput={(e) => setCustomFields(rule, 'fields', stripChips, e.currentTarget.value)}
              placeholder={t('custom_fields')}
            />
          {:else if rule.type === 'timezone'}
            <div class="rule-fields">
              <input class="input grow" bind:value={rule.target} placeholder={t('target_timezone')} />
              <input class="input grow" bind:value={rule.defaultTz} placeholder={t('default_timezone')} />
            </div>
          {:else if rule.type === 'expire'}
            <label class="inline">
              {t('drop_older_than')}
              <input class="input narrow" type="number" min="1" bind:value={rule.days} /> {t('days')}
            </label>
          {/if}
        </div>
      {/each}
      <div class="card-foot">
        <button type="button" class="button" onclick={runPreview} disabled={previewing}>
          {previewing ? t('previewing') : t('preview')}
        </button>
      </div>
    </section>

    <section class="card">
      <label class="check head-check">
        <input type="checkbox" bind:checked={advancedEnabled} /> <h2>{t('advanced')}</h2>
      </label>
      {#if advancedEnabled}
        <div class="row wrap">
          <label class="field">
            <span>{t('cache_ttl')}</span>
            <input class="input narrow" type="number" min="0" bind:value={ttlSeconds} />
          </label>
          <label class="field grow">
            <span>{t('basic_auth_user')}</span>
            <input class="input" bind:value={basicAuthUser} placeholder={t('basic_auth_disable_hint')} />
          </label>
          <label class="field grow">
            <span>{t('basic_auth_password')}</span>
            <input
              class="input"
              type="password"
              bind:value={basicAuthPassword}
              autocomplete="new-password"
              placeholder={feed?.basicAuthEnabled ? t('unchanged') : ''}
            />
          </label>
        </div>
      {/if}
    </section>

    <section class="card">
      <h2>{t('notifications')}</h2>
      <p class="muted">{t('notifications_desc')}</p>
      <div class="triggers">
        <span>{t('trigger_on')}</span>
        <label>
          <input type="checkbox" checked={notifyTriggers.includes('filter')} onchange={() => toggleTrigger('filter')} />
          {t('rule_filter')}
        </label>
        <label>
          <input type="checkbox" checked={notifyTriggers.includes('rename')} onchange={() => toggleTrigger('rename')} />
          {t('rule_rename')}
        </label>
      </div>

      <label class="check"><input type="checkbox" bind:checked={webhookEnabled} /> {t('enable_webhook')}</label>
      {#if webhookEnabled}
        <label class="field">
          <span>{t('webhook_url')}</span>
          <input class="input" bind:value={notifyWebhook} placeholder="https://…" />
        </label>
      {/if}

      <label class="check"><input type="checkbox" bind:checked={ntfyEnabled} /> {t('enable_ntfy')}</label>
      {#if ntfyEnabled}
        <div class="row wrap">
          <label class="field grow">
            <span>{t('ntfy_server')}</span>
            <input class="input" bind:value={notifyNtfyServer} placeholder="https://ntfy.sh" />
          </label>
          <label class="field grow">
            <span>{t('ntfy_topic')}</span>
            <input class="input" bind:value={notifyNtfyTopic} />
          </label>
        </div>
      {/if}

      <label class="check"><input type="checkbox" bind:checked={gotifyEnabled} /> {t('enable_gotify')}</label>
      {#if gotifyEnabled}
        <div class="row wrap">
          <label class="field grow">
            <span>{t('gotify_server')}</span>
            <input class="input" bind:value={notifyGotifyServer} placeholder="https://gotify.example.com" />
          </label>
          <label class="field grow">
            <span>{t('gotify_token')}</span>
            <input
              class="input"
              type="password"
              bind:value={notifyGotifyToken}
              autocomplete="new-password"
              placeholder={initial?.notifications.gotifyTokenSet ? t('unchanged') : ''}
            />
          </label>
        </div>
      {/if}
    </section>

    {#if error}<p class="error">{error}</p>{/if}

    <div class="actions">
      <button type="submit" class="button" disabled={saving}>
        {saving ? t('saving') : feed ? t('save_changes') : t('create_calendar')}
      </button>
      <a class="button button-secondary" href="/feeds">{t('cancel')}</a>
    </div>
  </form>

  {#if preview}
    <aside class="preview-panel" class:collapsed={!panelOpen}>
      <div class="panel-head">
        <h2>{t('preview')}</h2>
        <button type="button" class="linklike" onclick={() => (panelOpen = !panelOpen)}>
          {panelOpen ? t('hide_preview') : t('show_preview')}
        </button>
      </div>
      {#if panelOpen}
        <div class="week-nav">
          <button type="button" class="button button-secondary button-sm" onclick={() => weekOffset--}>‹ {t('prev_week')}</button>
          <span class="week-label">{tf('this_week', { date: currentWeekStart.toLocaleDateString(lang) })}</span>
          <button type="button" class="button button-secondary button-sm" onclick={() => weekOffset++}>{t('next_week')} ›</button>
        </div>

        <div class="diff">
          <div class="diff-col">
            <h3>{t('original')} <span class="badge">{weekOriginal.length}</span></h3>
            {#if weekOriginal.length === 0}
              <p class="muted">{t('no_events_week')}</p>
            {:else}
              <ul>
                {#each weekOriginal as e, i (i)}
                  <li><span class="when">{fmtWhen(e.start)}</span> {e.summary}</li>
                {/each}
              </ul>
            {/if}
          </div>
          <div class="diff-col transformed">
            <h3>{t('transformed')} <span class="badge badge-ok">{weekTransformed.length}</span></h3>
            {#if weekTransformed.length === 0}
              <p class="muted">{t('no_events_week')}</p>
            {:else}
              <ul>
                {#each weekTransformed as e, i (i)}
                  <li><span class="when">{fmtWhen(e.start)}</span> {e.summary}</li>
                {/each}
              </ul>
            {/if}
          </div>
        </div>
      {/if}
    </aside>
  {/if}
</div>

<style>
  .editor-layout {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1.35fr);
    gap: var(--space-5);
    align-items: start;
  }
  @media (max-width: 1024px) {
    .editor-layout {
      grid-template-columns: 1fr;
    }
  }
  form {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    min-width: 0;
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
  .card-foot {
    display: flex;
    padding-top: var(--space-4);
    border-top: 1px solid var(--separator);
  }
  .card-foot :global(.button) {
    width: 100%;
    padding: var(--space-3) var(--space-5);
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
  .row.wrap {
    flex-wrap: wrap;
    align-items: flex-end;
  }
  .grow {
    flex: 1;
    min-width: 0;
  }
  .narrow {
    width: 120px;
  }
  .source {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3);
    border: 1px solid var(--separator);
    border-radius: var(--radius-md);
  }
  .creds.disabled {
    opacity: 0.45;
  }
  .src-input {
    position: relative;
  }
  .src-input .input {
    padding-right: var(--space-7);
  }
  .src-status {
    position: absolute;
    right: var(--space-3);
    top: 50%;
    transform: translateY(-50%);
    display: grid;
    place-items: center;
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    cursor: default;
  }
  .src-status.ok {
    color: var(--success);
  }
  .src-status.error {
    color: var(--danger);
  }
  .spinner {
    width: 14px;
    height: 14px;
    border: 2px solid var(--separator);
    border-top-color: var(--accent);
    border-radius: var(--radius-full);
    animation: spin 0.7s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-secondary);
  }
  .head-check h2 {
    font-weight: var(--weight-semibold);
  }
  .rule {
    position: relative;
    border: 1px solid var(--separator);
    border-left: 3px solid var(--accent);
    border-radius: var(--radius-md);
    padding: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.02), transparent);
    transition:
      border-color var(--dur-fast) var(--ease),
      box-shadow var(--dur-fast) var(--ease),
      opacity var(--dur-fast) var(--ease),
      transform var(--dur-fast) var(--ease);
  }
  .rule:hover {
    box-shadow: var(--shadow-sm);
  }
  .rule.off {
    opacity: 0.5;
    border-left-color: var(--separator);
  }
  .rule.dragging {
    opacity: 0.4;
  }
  .rule.drag-over {
    border-color: var(--accent);
    box-shadow: var(--focus-ring);
    transform: translateY(-1px);
  }
  .rule-head {
    display: flex;
    gap: var(--space-2);
    align-items: center;
    flex-wrap: wrap;
  }
  .drag-handle {
    flex-shrink: 0;
    display: grid;
    place-items: center;
    width: 24px;
    height: 28px;
    color: var(--text-tertiary);
    cursor: grab;
    border-radius: var(--radius-sm);
    transition: color var(--dur-fast) var(--ease), background var(--dur-fast) var(--ease);
  }
  .drag-handle:hover {
    color: var(--text-secondary);
    background: rgba(255, 255, 255, 0.05);
  }
  .drag-handle:active {
    cursor: grabbing;
  }
  .drag-handle svg {
    width: 16px;
    height: 16px;
  }
  .move-btns {
    display: inline-flex;
    flex-direction: column;
    gap: 2px;
    flex-shrink: 0;
  }
  .move {
    width: 20px;
    height: 14px;
    display: grid;
    place-items: center;
    border: 1px solid var(--separator);
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-tertiary);
    font-size: 8px;
    line-height: 1;
    cursor: pointer;
    transition: all var(--dur-fast) var(--ease);
  }
  .move:hover:not(:disabled) {
    color: var(--accent);
    border-color: var(--accent);
  }
  .move:disabled {
    opacity: 0.35;
    cursor: not-allowed;
  }
  .rule-num {
    flex-shrink: 0;
    width: 24px;
    height: 24px;
    display: grid;
    place-items: center;
    border-radius: var(--radius-full);
    background: var(--accent);
    color: var(--accent-text);
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
  }
  .rule-head select {
    width: 160px;
  }
  .toggle {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    font-size: var(--text-xs);
    color: var(--text-secondary);
    margin-left: auto;
  }
  .rule-desc {
    margin: 0;
    color: var(--text-tertiary);
    font-size: var(--text-xs);
  }
  .rule-fields {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .chips-label {
    font-size: var(--text-xs);
    color: var(--text-tertiary);
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .chip {
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-full);
    border: 1px solid var(--separator);
    background: transparent;
    color: var(--text-secondary);
    font-size: var(--text-sm);
    cursor: pointer;
    transition: all var(--dur-fast) var(--ease);
  }
  .chip.on {
    background: var(--accent);
    color: var(--accent-text);
    border-color: var(--accent);
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
    flex-shrink: 0;
  }
  .icon:hover {
    color: var(--danger);
    border-color: var(--danger);
  }
  .actions {
    display: flex;
    gap: var(--space-3);
    align-items: center;
    flex-wrap: wrap;
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
    flex-wrap: wrap;
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

  /* Preview panel */
  .preview-panel {
    position: sticky;
    top: var(--space-5);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4);
    border: 1px solid var(--separator);
    border-radius: var(--radius-lg);
    background: var(--bg-elevated);
    max-height: calc(100vh - 2 * var(--space-5));
    overflow: auto;
  }
  .panel-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
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
    text-align: center;
    flex: 1;
  }
  .diff {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-4);
  }
  @media (max-width: 540px) {
    .diff {
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
  li {
    font-size: var(--text-sm);
    padding: var(--space-2) var(--space-3);
    background: var(--bg-base);
    border-radius: var(--radius-sm);
    border-left: 2px solid var(--separator);
    transition: border-color var(--dur-fast) var(--ease);
  }
  .diff-col.transformed li {
    border-left-color: var(--success);
  }
  .diff-col li:hover {
    border-left-color: var(--accent);
  }
  .when {
    color: var(--text-tertiary);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    margin-right: var(--space-2);
  }
</style>
