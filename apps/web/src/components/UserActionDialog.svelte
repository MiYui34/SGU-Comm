<script lang="ts">
  import { untrack } from "svelte";
  import { BAN_TIMES, TIMEOUT_TIMES, type ModerationAction } from "../lib/moderation";

  // Asks before a kick, ban or time-out: a reason (kick, ban) and a time
  // (ban, time-out). timedBans: bans can have a time (else for ever).

  let {
    kind,
    name,
    timedBans = false,
    onSubmit,
    onClose,
  }: {
    kind: "kick" | "ban" | "timeout";
    name: string;
    timedBans?: boolean;
    onSubmit: (a: ModerationAction) => void;
    onClose: () => void;
  } = $props();

  let dialog: HTMLDialogElement | undefined = $state();
  const k = untrack(() => kind);
  const times = k === "timeout" ? TIMEOUT_TIMES : untrack(() => timedBans) ? BAN_TIMES : [];
  let reason = $state("");
  let seconds = $state<number>(times[0]?.seconds ?? 0);
  $effect(() => dialog?.showModal());

  const title = { kick: "Kick", ban: "Ban", timeout: "Time out" }[k];
  const submit = (event: SubmitEvent) => {
    event.preventDefault();
    const r = reason.trim();
    onSubmit(k === "kick" ? { kind: k, reason: r } : k === "ban" ? { kind: k, reason: r, seconds } : { kind: k, seconds });
    dialog?.close();
  };
</script>

<dialog bind:this={dialog} onclose={onClose} aria-labelledby="user-action-title">
  <form onsubmit={submit}>
    <header>
      <h2 id="user-action-title">{title} {name}</h2>
      <button type="button" class="close" onclick={() => dialog?.close()} aria-label="Close">×</button>
    </header>
    <p>
      {k === "kick"
        ? `${name} leaves the server and can come back.`
        : k === "ban"
          ? `${name} leaves the server and cannot come back${times.length ? " for this time" : ""}.`
          : `${name} cannot write, react or speak for this time.`}
    </p>
    {#if times.length}
      <label>
        {k === "ban" ? "Ban for" : "Time out for"}
        <select bind:value={seconds}>
          {#each times as t (t.seconds)}<option value={t.seconds}>{t.label}</option>{/each}
        </select>
      </label>
    {/if}
    {#if k !== "timeout"}
      <label>
        Reason (optional)
        <!-- svelte-ignore a11y_autofocus -->
        <input bind:value={reason} maxlength="200" autofocus />
      </label>
    {/if}
    <footer>
      <button type="button" class="secondary" onclick={() => dialog?.close()}>Cancel</button>
      <button type="submit" class="danger">{title}</button>
    </footer>
  </form>
</dialog>

<style>
  dialog {
    width: min(440px, 92vw);
    padding: 24px;
    border: none;
    border-radius: 8px;
    background: var(--bg-main);
    color: var(--text);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 0.6);
  }
  header,
  footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  header {
    margin-bottom: 16px;
  }
  footer {
    justify-content: flex-end;
    gap: 8px;
    margin-top: 8px;
  }
  h2 {
    margin: 0;
    font-size: 18px;
  }
  .close {
    font-size: 24px;
    line-height: 1;
    color: var(--text-muted);
  }
  p {
    margin: 0 0 16px;
    font-size: 14px;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin: 0 0 16px;
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  input,
  select {
    padding: 10px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    font: inherit;
    text-transform: none;
  }
  footer button {
    padding: 10px 16px;
    border-radius: 4px;
    color: white;
    font-weight: 600;
  }
  footer .secondary {
    background: none;
    color: var(--text);
  }
  footer .danger {
    background: var(--danger);
  }
</style>
