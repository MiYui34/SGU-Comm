import { untrack } from "svelte";
import type { Connection } from "./connection.svelte";
import { unreadTextIds } from "./textChannels";

// Text channel behaviour of the client view: load history once per channel,
// page back, track what was seen. Call during component setup (it uses
// effects). `currentId` is the open text channel, or null.
export const createTextView = (connection: () => Connection, currentId: () => number | null) => {
  const text = $derived(connection().text);
  const requested = new Set<number>();
  let seen = $state<Record<number, number>>({});
  let loadingOlder = $state(false);

  $effect(() => {
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
      if ((seen[id] ?? 0) < newest) seen = { ...seen, [id]: newest };
    });
  });

  const unread = $derived(unreadTextIds(text, seen));

  return {
    get unread() {
      return unread;
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
