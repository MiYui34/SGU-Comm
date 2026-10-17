import { listKey, replaceTyped } from "./suggest";
import type { Suggestion } from "../components/Suggestions.svelte";

// A text field with a suggestion list for the word being typed (see
// lib/suggest): the message box ("@", "#", ":") and the search box
// ("from:", "in:"). find: the typed word before the caret; build: its
// suggestions; setText: puts the new text and caret into the field.
// Escape hides the list until the typed word changes; leaving the field
// hides it until the next key or click there.
export const createCombobox = <T extends { start: number }>(o: {
  id: string;
  text: () => string;
  input: () => HTMLInputElement | HTMLTextAreaElement | undefined;
  find: (text: string, caret: number) => T | null;
  build: (typed: T) => Suggestion[];
  setText: (text: string, caret: number) => void | Promise<void>;
  onPick?: (s: Suggestion) => void;
}) => {
  let caret = $state(0);
  let selected = $state(0);
  let dismissed = $state<number | null>(null);
  let focused = $state(false);
  const typed = $derived(o.find(o.text(), caret));
  const items = $derived(focused && typed && typed.start !== dismissed ? o.build(typed) : []);
  const active = $derived(Math.min(selected, items.length - 1));

  const update = () => {
    caret = o.input()?.selectionStart ?? o.text().length;
    selected = 0;
    focused = true;
    if (!typed || typed.start !== dismissed) dismissed = null;
  };
  const pick = async (s: Suggestion) => {
    if (!typed) return;
    o.onPick?.(s);
    const next = replaceTyped(o.text(), typed.start, caret, s.insert);
    caret = next.caret;
    focused = true;
    await o.setText(next.text, next.caret);
  };

  return {
    get typed() {
      return typed;
    },
    get items() {
      return items;
    },
    get active() {
      return active;
    },
    pick,
    // The caret moved (input, click, keys).
    update,
    onkeyup: (e: KeyboardEvent) => {
      // Up and Down move in the open list; other keys may move the caret.
      if (!(items.length && (e.key === "ArrowUp" || e.key === "ArrowDown"))) update();
    },
    // true: the key was for the list. Not while an input method composes
    // (229: Safari, after compositionend), not with Shift (Shift+Tab) or
    // Alt (Alt+Up/Down changes the channel).
    onkeydown: (e: KeyboardEvent) => {
      if (!items.length || e.isComposing || e.keyCode === 229 || e.shiftKey || e.altKey) return false;
      const k = listKey(e.key, items.length, active);
      if (!k) return false;
      e.preventDefault();
      if (k === "pick") void pick(items[active]!);
      else if (k === "close") dismissed = typed?.start ?? null;
      else selected = k.active;
      return true;
    },
    onblur: () => (focused = false),
    // Spread on the field.
    get aria() {
      const open = items.length > 0;
      return {
        role: "combobox",
        "aria-autocomplete": "list",
        "aria-expanded": open,
        "aria-controls": open ? o.id : undefined,
        "aria-activedescendant": open ? `${o.id}-${active}` : undefined,
      } as const;
    },
  };
};
