// An account may be open in several tabs of one browser (see the server's
// textonly.go); each would show the same desktop notification. Only the
// tab used last shows them, with its own rules (no notification for the
// chat open in it). Tabs tell each other over a BroadcastChannel per
// server and account. A tab that ends without a word (a crash) keeps the
// others quiet until one of them is used again.

export type TabMessage = { type: "hello" | "active"; id: string; at: number } | { type: "bye"; id: string };

// When each known tab was used last, by id.
export type Tabs = Readonly<Record<string, number>>;

export const tabsAfter = (tabs: Tabs, m: TabMessage): Tabs => {
  if (m.type !== "bye") return { ...tabs, [m.id]: m.at };
  const { [m.id]: _, ...rest } = tabs;
  return rest;
};

// The tab leads: no other was used later (ties: the smaller id).
export const leads = (tabs: Tabs, me: string): boolean => {
  const mine = tabs[me] ?? -Infinity;
  return Object.entries(tabs).every(([id, at]) => id === me || at < mine || (at === mine && me < id));
};

// The leader of the tabs on one channel name. touch: this tab was used
// (focused or shown); a new tab counts as used.
export const createNotifyLeader = (name: string, now = Date.now) => {
  const id = crypto.randomUUID();
  let tabs: Tabs = {};
  let channel = typeof BroadcastChannel === "undefined" ? null : new BroadcastChannel(name);
  const post = (m: TabMessage) => {
    tabs = tabsAfter(tabs, m);
    channel?.postMessage(m);
  };
  channel?.addEventListener("message", (e: MessageEvent<TabMessage>) => {
    tabs = tabsAfter(tabs, e.data);
    // A new tab learns about the others.
    if (e.data.type === "hello" && tabs[id] !== undefined) channel?.postMessage({ type: "active", id, at: tabs[id] });
  });
  post({ type: "hello", id, at: now() });
  return {
    leads: () => leads(tabs, id),
    touch: () => post({ type: "active", id, at: now() }),
    // Once (pagehide, then the component's end).
    close: () => {
      post({ type: "bye", id });
      channel?.close();
      channel = null;
    },
  };
};
