<script lang="ts">
  import { api, ApiError } from '$lib/api';

  let email = $state('');
  let submitting = $state(false);
  let done = $state(false);
  let error = $state<string | null>(null);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    submitting = true;
    error = null;
    try {
      await api.requestPasswordReset(email);
      done = true;
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Request failed';
    } finally {
      submitting = false;
    }
  }
</script>

<div class="auth">
  <div class="card">
    <h1>Reset password</h1>
    {#if done}
      <p class="subtitle">If an account exists for that address, a reset link is on its way.</p>
      <a class="button button-secondary" href="/login">Back to sign in</a>
    {:else}
      <p class="subtitle">Enter your email and we'll send a reset link.</p>
      <form onsubmit={submit}>
        <label>
          <span>Email</span>
          <input class="input" type="email" bind:value={email} autocomplete="username" required />
        </label>
        {#if error}<p class="error">{error}</p>{/if}
        <button class="button" type="submit" disabled={submitting}>
          {submitting ? 'Sending…' : 'Send reset link'}
        </button>
      </form>
      <p class="hint"><a href="/login">Back to sign in</a></p>
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
  .subtitle {
    margin: 0;
    color: var(--text-secondary);
    font-size: var(--text-sm);
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
  .hint {
    margin: 0;
    text-align: center;
    color: var(--text-tertiary);
    font-size: var(--text-sm);
  }
</style>
