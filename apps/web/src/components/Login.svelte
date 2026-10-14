<script lang="ts">
  import type { Credentials } from "../lib/connection.svelte";
  import { safeGet, safeSet } from "../lib/storage";

  let { error, onConnect }: { error: string | null; onConnect: (c: Credentials) => void } =
    $props();

  let username = $state(safeGet(localStorage, "username") ?? "");
  let password = $state("");

  const submit = (event: SubmitEvent) => {
    event.preventDefault();
    if (!username.trim()) return;
    safeSet(localStorage, "username", username.trim());
    onConnect({ username: username.trim(), password });
  };
</script>

<div class="screen">
  <form onsubmit={submit}>
    <h1>Welcome to Kallisti</h1>
    <p class="sub">Join the voice server</p>
    <label>
      Name
      <input bind:value={username} required maxlength="64" autocomplete="nickname" />
    </label>
    <label>
      Server password <span class="optional">(optional)</span>
      <input bind:value={password} type="password" autocomplete="current-password" />
    </label>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <button type="submit">Connect</button>
  </form>
</div>

<style>
  .screen {
    display: grid;
    place-items: center;
    height: 100%;
    background: var(--bg-rail);
  }
  form {
    display: flex;
    flex-direction: column;
    gap: 16px;
    width: min(400px, 90vw);
    padding: 32px;
    border-radius: 8px;
    background: var(--bg-main);
  }
  h1 {
    margin: 0;
    font-size: 24px;
    text-align: center;
  }
  .sub {
    margin: -8px 0 8px;
    color: var(--text-muted);
    text-align: center;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .optional {
    text-transform: none;
    font-weight: normal;
  }
  input {
    padding: 10px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    outline: none;
  }
  button {
    padding: 12px;
    border-radius: 4px;
    background: var(--accent);
    color: white;
    font-weight: 600;
  }
  .error {
    margin: 0;
    color: var(--danger);
  }
</style>
