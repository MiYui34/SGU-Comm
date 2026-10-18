export type Closable = { close(): void };

// One encoder per talk spurt, like the desktop Mumble client resets its
// encoder: no state carries over from the previous spurt. The old encoder is
// closed once its last input has come out.
//
// Outputs are matched to inputs in order (R: what the caller keeps per
// input, such as its timestamp and terminator flag), not by the output's
// timestamp: Firefox shifts those. An output after the last input (Chrome's
// flush() adds one for the lookahead) gets nothing and is dropped.
export const createEncoderRotation = <E extends Closable, R>(make: () => E) => {
  const pending = new Map<E, R[]>(); // inputs not out yet, per encoder
  let current = make();
  let used = false;
  const retired = new Set<E>();

  const closeIfDone = (e: E) => {
    if (retired.has(e) && !pending.get(e)?.length) {
      e.close();
      retired.delete(e);
      pending.delete(e);
    }
  };

  const renew = () => {
    retired.add(current);
    closeIfDone(current);
    current = make();
    used = false;
  };

  return {
    current: () => current,
    encoded: (e: E, input: R) => {
      pending.set(e, [...(pending.get(e) ?? []), input]);
      if (e === current) used = true;
    },
    output: (e: E): R | undefined => {
      const input = pending.get(e)?.shift();
      closeIfDone(e);
      return input;
    },
    startSpurt: () => {
      if (used) renew();
    },
    // A new encoder now, for example for new settings.
    renew,
    closeAll: () => {
      for (const e of [...retired, current]) e.close();
      retired.clear();
      pending.clear();
    },
  };
};
