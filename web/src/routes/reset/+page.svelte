<script lang="ts">
  import { t } from '$lib/i18n';
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api, ApiError } from '$lib/api';

  let token = $state('');
  let password = $state('');
  let submitting = $state(false);
  let error = $state<string | null>(null);

  onMount(() => {
    token = $page.url.searchParams.get('token') ?? '';
  });

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    submitting = true;
    error = null;
    try {
      await api.confirmPasswordReset(token, password);
      await goto('/login');
    } catch (err) {
      error = err instanceof ApiError ? err.message : t('save_failed');
    } finally {
      submitting = false;
    }
  }
</script>

<div class="auth">
  <div class="card">
    <h1>{t('set_new_password')}</h1>
    {#if !token}
      <p class="error">{t('reset_token_missing')}</p>
      <a class="button button-secondary" href="/reset/request">{t('reset_password')}</a>
    {:else}
      <form onsubmit={submit}>
        <label>
          <span>{t('new_password')}</span>
          <input
            class="input"
            type="password"
            bind:value={password}
            autocomplete="new-password"
            minlength="8"
            required
          />
        </label>
        {#if error}<p class="error">{error}</p>{/if}
        <button class="button" type="submit" disabled={submitting}>
          {submitting ? t('saving') : t('set_password')}
        </button>
      </form>
    {/if}
  </div>
</div>

<style>
  .auth {
    display: flex;
    justify-content: center;
    padding-top: var(--space-7);
  }
  .card {
    width: 100%;
    max-width: 380px;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  h1 {
    font-size: var(--text-2xl);
    margin: 0;
  }
  form {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  label {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-secondary);
  }
  .error {
    margin: 0;
    color: var(--danger);
    font-size: var(--text-sm);
  }
</style>
