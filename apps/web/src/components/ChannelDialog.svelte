<script lang="ts">
  import { untrack } from "svelte";
  import type { ChannelForm, ChannelKind } from "../lib/channelForm";
  import type { ListenOnly, Speaker } from "../lib/listenOnly";
  import { SLOW_MODES, slowModeLabel } from "../lib/slowMode";

  // Create, edit or delete a channel or a text category.

  let {
    form,
    onSubmit,
    onClose,
  }: {
    form: ChannelForm;
    onSubmit: (v: {
      kind: ChannelKind;
      name: string;
      description: string;
      temporary: boolean;
      listenOnly?: ListenOnly;
      private?: boolean;
      slowMode?: number;
      category?: number;
    }) => void;
    onClose: () => void;
  } = $props();

  let dialog: HTMLDialogElement | undefined = $state();
  // The fields start from the form; the dialog is made anew for each one.
  const start = untrack(() => form);
  // Staff making a text channel may make a category instead.
  let kind = $state(start.kind);
  const noun = $derived(kind === "category" ? "category" : "channel");
  // A text channel's category (0: none); the choice is for staff.
  const categories = start.mode === "delete" ? undefined : start.categories;
  const startCategory = start.mode === "delete" ? undefined : start.category;
  let category = $state(startCategory ?? 0);
  let name = $state(start.mode === "create" ? "" : start.name);
  let description = $state(start.mode === "edit" ? start.description : "");
  // Voice channels with the built-in server.
  const startListenOnly = start.mode === "edit" ? start.listenOnly : undefined;
  let listenOnly = $state(startListenOnly?.on ?? false);
  let speakers = $state<Speaker[]>(startListenOnly?.speakers ?? []);
  const addable = $derived((startListenOnly?.users ?? []).filter((u) => !speakers.some((s) => s.id === u.id)));
  const addSpeaker = (event: Event & { currentTarget: HTMLSelectElement }) => {
    const u = addable.find((a) => String(a.id) === event.currentTarget.value);
    if (u) speakers = [...speakers, u];
    event.currentTarget.value = "";
  };
  // Admins and moderators (built-in server): a private channel.
  const canPrivate = start.mode === "create" ? start.canPrivate === true : start.mode === "edit" && !!start.private;
  let isPrivate = $state(start.mode === "edit" ? (start.private?.on ?? false) : false);
  // Text channels (built-in server): undefined, no field.
  const startSlowMode = start.mode === "edit" ? start.slowMode : undefined;
  let slowMode = $state(startSlowMode ?? 0);
  // Users who may only make temporary voice channels get one of those.
  const temporary = $derived(form.mode === "create" && form.temporaryOnly && kind === "voice" && !isPrivate);
  $effect(() => dialog?.showModal());

  const title = $derived(
    form.mode === "create" && form.parent ? "Create subchannel" : `${form.mode === "create" ? "Create" : form.mode === "edit" ? "Edit" : "Delete"} ${noun}`,
  );
  const submit = (event: SubmitEvent) => {
    event.preventDefault();
    if (form.mode !== "delete" && !name.trim()) return;
    onSubmit({
      kind,
      name: name.trim(),
      description,
      temporary,
      ...(startListenOnly ? { listenOnly: { on: listenOnly, speakers } } : {}),
      ...(canPrivate && kind !== "category" ? { private: isPrivate } : {}),
      ...(startSlowMode !== undefined ? { slowMode } : {}),
      ...(startCategory !== undefined && kind === "text" ? { category } : {}),
    });
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
          : form.kind === "category"
            ? "Its channels stay, without a category."
            : "Its messages are kept and come back if a channel of this name is made again."}
      </p>
    {:else}
      {#if start.mode === "create" && start.canCategory}
        <div class="kinds" role="radiogroup" aria-label="What to create">
          <label class="check"><input type="radio" bind:group={kind} value="text" /> Text channel</label>
          <label class="check"><input type="radio" bind:group={kind} value="category" /> Category</label>
        </div>
      {/if}
      <label>
        {kind === "category" ? "Category" : "Channel"} name
        <!-- svelte-ignore a11y_autofocus -->
        <input bind:value={name} maxlength={kind === "voice" ? 100 : 32} required autofocus readonly={start.mode === "edit" && start.topicOnly}
          placeholder={kind === "text" ? "new-channel" : kind === "category" ? "New category" : "New channel"} />
      </label>
      {#if categories?.length && kind === "text"}
        <label>
          Category
          <select bind:value={category}>
            <option value={0}>No category</option>
            {#each categories as c (c.id)}<option value={c.id}>{c.name}</option>{/each}
          </select>
        </label>
      {/if}
      {#if form.mode === "edit" && kind !== "category"}
        <label>
          {kind === "text" ? "Topic" : "Description"}
          <textarea bind:value={description} rows="3" maxlength={kind === "text" ? 200 : undefined}></textarea>
        </label>
      {/if}
      {#if startSlowMode !== undefined}
        <label>
          Slow mode
          <select bind:value={slowMode}>
            {#each SLOW_MODES as s (s)}<option value={s}>{slowModeLabel(s)}</option>{/each}
          </select>
          <span class="note hint">How often each member may post here. Admins and moderators (also this channel's) are exempt.</span>
        </label>
      {/if}
      {#if canPrivate && kind !== "category"}
        <label class="check">
          <input type="checkbox" bind:checked={isPrivate} />
          <span>
            Private channel
            <span class="note">Only its members, admins and moderators see it. Add members in the channel menu (Members…).</span>
          </span>
        </label>
      {/if}
      {#if startListenOnly}
        <label class="check">
          <input type="checkbox" bind:checked={listenOnly} />
          <span>
            Listen only
            <span class="note">Only the people below can speak. Everyone else listens, also admins and moderators.</span>
          </span>
        </label>
        {#if listenOnly}
          <div class="speakers">
            <span class="label">Who can speak</span>
            {#if speakers.length}
              <ul>
                {#each speakers as s (s.id)}
                  <li>
                    {s.name}
                    <button type="button" aria-label="Remove {s.name}" onclick={() => (speakers = speakers.filter((x) => x.id !== s.id))}>×</button>
                  </li>
                {/each}
              </ul>
            {/if}
            {#if addable.length}
              <select aria-label="Add someone who can speak" value="" onchange={addSpeaker}>
                <option value="" disabled>Add someone (registered and online)…</option>
                {#each addable as u (u.id)}<option value={String(u.id)}>{u.name}</option>{/each}
              </select>
            {:else if !speakers.length}
              <p class="note">Nobody can speak. You can add registered users who are online.</p>
            {/if}
          </div>
        {/if}
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
    padding-bottom: 16px;
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
  .check {
    flex-direction: row;
    align-items: flex-start;
    gap: 10px;
    font-size: 14px;
    font-weight: normal;
    text-transform: none;
    color: var(--text);
  }
  .check input {
    margin-top: 3px;
  }
  .kinds {
    display: flex;
    gap: 24px;
  }
  .check .note {
    display: block;
    font-size: 12px;
  }
  .hint {
    font-weight: normal;
    text-transform: none;
  }
  .speakers {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin: 0 0 16px;
  }
  .label {
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .speakers ul {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .speakers li {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 4px 4px 10px;
    border-radius: 12px;
    background: var(--bg-rail);
    font-size: 14px;
  }
  .speakers li button {
    padding: 0 6px;
    color: var(--text-muted);
  }
  select {
    padding: 10px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    font: inherit;
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
