<script lang="ts">
  import type { Registered } from "@kallisti/protocol";
  import { untrack } from "svelte";
  import { memberCandidates, type Member } from "../lib/privateChannels";
  import { expiryChoices, inviteExpiry, inviteUses, useChoices, type Invite } from "../lib/invites";

  // The members of a private channel, for admins, moderators and the
  // channel's moderators: remove them, or add registered users (the server
  // checks again). Invite links apply at once (not on Save). moderators:
  // the channel's moderators instead (staff appoint them), no links.
  let {
    name,
    members,
    registered,
    invites = null,
    link = null,
    moderators = false,
    onLoad,
    onSubmit,
    onCreateInvite = () => {},
    onDeleteInvite = () => {},
    onClose,
  }: {
    name: string;
    members: readonly Member[];
    // Registered users and invite links (null: not loaded yet; onLoad asks
    // for them).
    registered: readonly Registered[] | null;
    invites?: readonly Invite[] | null;
    // A link just made: shown only now.
    link?: string | null;
    moderators?: boolean;
    onLoad: () => void;
    onSubmit: (members: Member[]) => void;
    onCreateInvite?: (seconds: number, uses: number) => void;
    onDeleteInvite?: (id: number) => void;
    onClose: () => void;
  } = $props();
  const noun = untrack(() => (moderators ? "moderator" : "member"));

  let dialog: HTMLDialogElement | undefined = $state();
  // The list starts from the current members; Save sends it.
  let list = $state<Member[]>(untrack(() => [...members]));
  const addable = $derived(memberCandidates(registered, list));
  $effect(() => dialog?.showModal());
  $effect(() => untrack(onLoad));

  let seconds = $state<number>(expiryChoices[0].seconds);
  let uses = $state<number>(useChoices[0].uses);
  let copied = $state<string | null>(null); // the link copied
  const copy = (text: string) => void navigator.clipboard?.writeText(text).then(() => (copied = text));
  const now = Date.now();

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
      <h2 id="members-dialog-title">{moderators ? "Moderators" : "Members"} of {name}</h2>
      <button type="button" class="close" onclick={() => dialog?.close()} aria-label="Close">×</button>
    </header>
    <p class="note">
      {moderators
        ? "They delete and pin messages, set the topic and slow mode, mute, move and time out people here and in its subchannels, and manage the members of a private channel. Never admins or moderators."
        : "Only members, admins and moderators see this channel."}
    </p>
    {#if list.length}
      <ul aria-label={moderators ? "Moderators" : "Members"}>
        {#each list as m (m.id)}
          <li>
            <span class="ellipsis">{m.name}</span>
            <button type="button" aria-label="Remove {m.name}" onclick={() => (list = list.filter((x) => x.id !== m.id))}>×</button>
          </li>
        {/each}
      </ul>
    {:else}
      <p>No {noun}s yet.</p>
    {/if}
    {#if registered === null}
      <p class="note">Loading registered users…</p>
    {:else if addable.length}
      <select aria-label="Add a {noun}" value="" onchange={add}>
        <option value="" disabled>Add a {noun} (registered users)…</option>
        {#each addable as u (u.id)}<option value={String(u.id)}>{u.name}</option>{/each}
      </select>
    {:else}
      <p class="note">All registered users are {noun}s.</p>
    {/if}
    {#if !moderators}
      <h3>Invite links</h3>
      {#if link}
        <div class="row">
          <input readonly value={link} aria-label="New invite link" onfocus={(e) => e.currentTarget.select()} />
          <button type="button" class="action" onclick={() => copy(link)}>{copied === link ? "Copied" : "Copy"}</button>
        </div>
        <p class="note">Copy it now: it is shown only once.</p>
      {/if}
      <div class="row">
        <select aria-label="Valid for" bind:value={seconds}>
          {#each expiryChoices as c (c.seconds)}<option value={c.seconds}>{c.label}</option>{/each}
        </select>
        <select aria-label="Uses" bind:value={uses}>
          {#each useChoices as c (c.uses)}<option value={c.uses}>{c.label}</option>{/each}
        </select>
        <button type="button" class="action" onclick={() => onCreateInvite(seconds, uses)}>Create link</button>
      </div>
      {#if invites === null}
        <p class="note">Loading invite links…</p>
      {:else if invites.length}
        <ul class="links" aria-label="Active invite links">
          {#each invites as i (i.id)}
            <li>
              <span class="ellipsis">{inviteUses(i)}, {inviteExpiry(i.expires, now)}, by {i.creator}</span>
              <button type="button" aria-label="Delete the link by {i.creator}, {inviteUses(i)}" onclick={() => onDeleteInvite(i.id)}>×</button>
            </li>
          {/each}
        </ul>
      {:else}
        <p class="note">No active invite links.</p>
      {/if}
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
    padding-bottom: 8px;
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
  h3 {
    margin: 16px 0 8px;
    font-size: 14px;
  }
  .row {
    display: flex;
    gap: 8px;
    margin-bottom: 8px;
  }
  .row > * {
    min-width: 0;
  }
  .row input {
    flex: 1;
    padding: 10px;
    border: none;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
    font: inherit;
  }
  .action {
    flex: none;
    padding: 10px 12px;
    border-radius: 4px;
    background: var(--bg-rail);
    color: var(--text);
  }
  .links {
    flex-direction: column;
  }
  .links li {
    justify-content: space-between;
    border-radius: 4px;
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
