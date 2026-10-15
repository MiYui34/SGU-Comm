import { untrack } from "svelte";
import type { Connection } from "./connection.svelte";
import { mentionCounts } from "./mentions";
import { unreadTextIds } from "./textChannels";

// Text channel behaviour of the client view: load history once per channel,
// page back, track what was seen. Call during component setup (it uses
// effects). `currentId` is the open text channel, or null.
// `me` is our name, for mentions.
const MARK_DELAY_MS = 2000;

export const createTextView = (connection: () => Connection, currentId: () => number | null, me: () => string) => {
  const text = $derived(connection().text);
  const requested = new Set<number>();
  let seen = $state<Record<number, number>>({});
  let loadingOlder = $state(false);

  // In one effect with the load, so the order is fixed: after a resync the
  // history is gone and the open channel loads again.
  let seenResyncs = 0;
  $effect(() => {
    if (text.resyncs !== seenResyncs) {
      seenResyncs = text.resyncs;
      requested.clear();
    }
    const id = currentId();
    if (id !== null && !text.loaded[id] && !requested.has(id)) {
      requested.add(id);
      connection().requestHistory(id);
    }
  });

  // An error answers some request: allow retries, end a pending page load.
  $effect(() => {
    if (!text.lastError) return;
    untrack(() => {
      requested.clear();
      loadingOlder = false;
    });
  });

  // New messages in the open channel: a page load is done, and they count
  // as seen.
  $effect(() => {
    const id = currentId();
    if (id === null) return;
    const newest = text.messages[id]?.at(-1)?.id ?? 0;
    void text.messages[id]?.length;
    untrack(() => {
      loadingOlder = false;
      if ((read[id] ?? 0) >= newest) return;
      seen = { ...seen, [id]: newest };
      queueMark(id, newest);
    });
  });

  // Read marks go to the server (registered users keep them), at most every
  // few seconds. After a reconnect, marks the server missed go again.
  const pending = new Map<number, number>();
  let flush: ReturnType<typeof setTimeout> | undefined;
  const queueMark = (id: number, message: number) => {
    pending.set(id, Math.max(pending.get(id) ?? 0, message));
    flush ??= setTimeout(() => {
      flush = undefined;
      for (const [channel, m] of pending) connection().markRead(channel, m);
      pending.clear();
    }, MARK_DELAY_MS);
  };
  $effect(() => {
    const reads = text.reads;
    untrack(() => {
      for (const [id, n] of Object.entries(seen)) if (n > (reads[Number(id)] ?? 0)) queueMark(Number(id), n);
    });
  });

  // What was read: here, or before (the server's read marks).
  const read = $derived.by(() => {
    const out: Record<number, number> = { ...text.reads };
    for (const [id, n] of Object.entries(seen)) out[Number(id)] = Math.max(out[Number(id)] ?? 0, n);
    return out;
  });
  const unread = $derived(unreadTextIds(text, read));
  // Unseen mentions of us per channel (text channels and voice chats).
  const mentions = $derived(mentionCounts(text, read, me()));

  return {
    get unread() {
      return unread;
    },
    get mentions() {
      return mentions;
    },
    get loadingOlder() {
      return loadingOlder;
    },
    loadOlder: () => {
      const id = currentId();
      const oldest = id === null ? undefined : text.messages[id]?.[0];
      if (id === null || !oldest || loadingOlder) return;
      loadingOlder = true;
      connection().requestHistory(id, oldest.id);
    },
  };
};
