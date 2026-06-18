<script lang="ts">
  import '$lib/styles/global.css';

  let { children } = $props();
</script>

<div class="app">
  <header class="topbar">
    <a class="brand" href="/">Tidy<span class="brand-accent">DAV</span></a>
  </header>
  <main class="content">
    {@render children()}
  </main>
</div>

<style>
  .app {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
  }

  .topbar {
    position: sticky;
    top: 0;
    z-index: 10;
    display: flex;
    align-items: center;
    height: 56px;
    padding: 0 var(--space-5);
    border-bottom: 1px solid var(--separator);
    background: var(--bg-overlay);
    backdrop-filter: blur(var(--blur));
    -webkit-backdrop-filter: blur(var(--blur));
  }

  .brand {
    font-size: var(--text-lg);
    font-weight: var(--weight-semibold);
    letter-spacing: -0.02em;
    color: var(--text-primary);
  }

  .brand-accent {
    color: var(--accent);
  }

  .content {
    flex: 1;
    width: 100%;
    max-width: 960px;
    margin: 0 auto;
    padding: var(--space-7) var(--space-5);
  }
</style>
