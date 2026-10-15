<script lang="ts">
  import { untrack } from "svelte";
  import type { ChannelForm, ChannelKind } from "../lib/channelForm";

  // Create, edit or delete a channel.

  let {
    form,
    onSubmit,
    onClose,
  }: {
    form: ChannelForm;
    onSubmit: (v: { kind: ChannelKind; name: string; description: string; temporary: boolean }) => void;
    onClose: () => void;
  } = $props();

  let dialog: HTMLDialogElement | undefined = $state();
  // The fields start from the form; the dialog is made anew for each one.
  const start = untrack(() => form);
  const kind = start.kind;
  let name = $state(start.mode === "create" ? "" : start.name);
  let description = $state(start.mode === "edit" ? start.description : "");
  // Users who may only make temporary voice channels get one of those.
  const temporary = $derived(form.mode === "create" && form.temporaryOnly && kind === "voice");
  $effect(() => dialog?.showModal());

  const title = $derived(
    form.mode === "create" ? "Create channel" : form.mode === "edit" ? "Edit channel" : "Delete channel",
  );
  const submit = (event: SubmitEvent) => {
    event.preventDefault();
    if (form.mode !== "delete" && !name.trim()) return;
    onSubmit({ kind, name: name.trim(), description, temporary });
    dialog?.close();
  };
</script>

<dialog bind:this={dialog} onclose={onClose} aria-labelledby="channel-dialog-title">
  <form onsubmit={submit}>
    <header>
      <h2 id="channel-dialog-title">{title}</h2>
      <button type="button" class="close" onclick={() => dialog?.close()} aria-label="Close">×</button>
    </header>
    {#if form.mode === "delete"}
      <p>
        Delete <strong>{form.kind === "text" ? `#${form.name}` : form.name}</strong>?
        {form.kind === "voice"
          ? "Its subchannels go too; people in them move up."
          : "Its messages are kept and come back if a channel of this name is made again."}
      </p>
    {:else}
      <label>
        Channel name
        <!-- svelte-ignore a11y_autofocus -->
        <input bind:value={name} maxlength={kind === "text" ? 32 : 100} required autofocus
          placeholder={kind === "text" ? "new-channel" : "New channel"} />
      </label>
      {#if form.mode === "edit"}
        <label>
          {kind === "text" ? "Topic" : "Description"}
          <textarea bind:value={description} rows="3" maxlength={kind === "text" ? 200 : undefined}></textarea>
        </label>
      {/if}
      {#if temporary}
        <p class="note">A temporary channel: it goes away when the last person leaves.</p>
      {/if}
    {/if}
    <footer>
      <button type="button" class="secondary" onclick={() => dialog?.close()}>Cancel</button>
      <button type="submit" class:danger={form.mode === "delete"}>
        {form.mode === "create" ? "Create" : form.mode === "edit" ? "Save" : "Delete"}
      </button>
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
  textarea {
    padding: 10px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    font: inherit;
    text-transform: none;
  }
  p {
    margin: 0 0 16px;
    font-size: 14px;
  }
  .note {
    color: var(--text-muted);
  }
  footer button {
    padding: 10px 16px;
    border-radius: 4px;
    background: var(--accent-strong);
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
