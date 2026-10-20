import { fade, idle, pending, SAVED_MS, settle, type Save } from "./saveFeedback";

// A control's save feedback (see lib/saveFeedback). run starts a request;
// only the newest one's answer counts. undo: put an immediate control
// (checkbox, select) back after a refusal.
export const createSave = () => {
  let state = $state<Save>(idle);
  let current = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  return {
    get state() {
      return state;
    },
    get busy() {
      return state.phase === "pending";
    },
    run: (request: () => Promise<unknown>, undo?: () => void) => {
      const mine = ++current;
      clearTimeout(timer);
      state = pending;
      void request().then(
        () => {
          if (mine !== current) return;
          state = settle(state);
          timer = setTimeout(() => (state = fade(state)), SAVED_MS);
        },
        (err: unknown) => {
          if (mine !== current) return;
          state = settle(state, err instanceof Error ? err.message : String(err));
          undo?.();
        },
      );
    },
  };
};
export type SaveFeedback = ReturnType<typeof createSave>;
