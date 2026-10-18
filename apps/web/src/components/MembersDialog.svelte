<script lang="ts">
  import type { Registered } from "@kallisti/protocol";
  import { untrack } from "svelte";
  import { memberCandidates, type Member } from "../lib/privateChannels";

  // The members of a private channel, for admins and moderators: remove
  // them, or add registered users (the server checks again).
  let {
    name,
    members,
    registered,
    onLoad,
    onSubmit,
    onClose,
  }: {
    name: string;
    members: readonly Member[];
    // Registered users (null: not loaded yet; onLoad asks for them).
    registered: readonly Registered[] | null;
    onLoad: () => void;
    onSubmit: (members: Member[]) => void;
    onClose: () => void;
  } = $props();

  let dialog: HTMLDialogElement | undefined = $state();
  // The list starts from the current members; Save sends it.
  let list = $state<Member[]>(untrack(() => [...members]));
  const addable = $derived(memberCandidates(registered, list));
  $effect(() => dialog?.showModal());
  $effect(() => untrack(onLoad));

  const add = (event: Event & { currentTarget: HTMLSelectElement }) => {
    const u = addable.find((a) => String(a.id) === event.currentTarget.value);
    if (u) list = [...list, u];
    event.currentTarget.value = "";
  };
  const submit = (event: SubmitEvent) => {
    event.preventDefault();
    onSubmit(list);
    dialog?.close();
  };
</script>

<dialog bind:this={dialog} onclose={onClose} aria-labelledby="members-dialog-title">
  <form onsubmit={submit}>
    <header>
      <h2 id="members-dialog-title">Members of {name}</h2>
      <button type="button" class="close" onclick={() => dialog?.close()} aria-label="Close">×</button>
    </header>
    <p class="note">Only members, admins and moderators see this channel.</p>
    {#if list.length}
      <ul aria-label="Members">
        {#each list as m (m.id)}
          <li>
            <span class="ellipsis">{m.name}</span>
            <button type="button" aria-label="Remove {m.name}" onclick={() => (list = list.filter((x) => x.id !== m.id))}>×</button>
          </li>
        {/each}
      </ul>
    {:else}
      <p>No members yet.</p>
    {/if}
    {#if registered === null}
      <p class="note">Loading registered users…</p>
    {:else if addable.length}
      <select aria-label="Add a member" value="" onchange={add}>
        <option value="" disabled>Add a member (registered users)…</option>
        {#each addable as u (u.id)}<option value={String(u.id)}>{u.name}</option>{/each}
      </select>
    {:else}
      <p class="note">All registered users are members.</p>
    {/if}
    <footer>
      <button type="button" class="secondary" onclick={() => dialog?.close()}>Cancel</button>
      <button type="submit">Save</button>
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
    margin-bottom: 8px;
  }
  footer {
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
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
    margin: 0 0 12px;
    font-size: 14px;
  }
  .note {
    color: var(--text-muted);
  }
  ul {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 0 0 12px;
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    gap: 4px;
    max-width: 100%;
    padding: 4px 4px 4px 10px;
    border-radius: 12px;
    background: var(--bg-rail);
    font-size: 14px;
  }
  li button {
    padding: 0 6px;
    color: var(--text-muted);
  }
  select {
    width: 100%;
    padding: 10px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    font: inherit;
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
</style>
