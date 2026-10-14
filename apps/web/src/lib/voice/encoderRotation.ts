export type Closable = { close(): void };

// One encoder per talk spurt, like the desktop Mumble client resets its
// encoder: no state carries over from the previous spurt. reset() would drop
// the old spurt's last packet (the terminator), and flush() emits an extra
// packet after it, so the old encoder is closed once its last output is out
// (one output per input, as in Chrome).
export const createEncoderRotation = <E extends Closable>(make: () => E) => {
  const pending = new Map<E, number>(); // encoded minus output, per encoder
  let current = make();
  let used = false;
  const retired = new Set<E>();

  const closeIfDone = (e: E) => {
    if (retired.has(e) && (pending.get(e) ?? 0) <= 0) {
      e.close();
      retired.delete(e);
      pending.delete(e);
    }
  };

  return {
    current: () => current,
    encoded: (e: E) => {
      pending.set(e, (pending.get(e) ?? 0) + 1);
      if (e === current) used = true;
    },
    output: (e: E) => {
      pending.set(e, (pending.get(e) ?? 0) - 1);
      closeIfDone(e);
    },
    startSpurt: () => {
      if (!used) return;
      retired.add(current);
      closeIfDone(current);
      current = make();
      used = false;
    },
    closeAll: () => {
      for (const e of [...retired, current]) e.close();
      retired.clear();
      pending.clear();
    },
  };
};
