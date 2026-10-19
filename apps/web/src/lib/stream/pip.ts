// Picture-in-picture of one stream at a time. Toggles run one after the
// other (a second fast click waits for the first), and the stream that
// floats is read from the browser's pictureInPictureElement: a toggle that
// failed or was overtaken cannot leave the button in the wrong state.

type PipDocument = { readonly pictureInPictureElement: Element | null; exitPictureInPicture: () => Promise<void> };

// enter: floats the stream's picture, its element (null: it failed).
// onChange: the stream that floats now (null: none).
export const createPip = (doc: PipDocument, enter: (session: number) => Promise<Element | null>, onChange: (session: number | null) => void) => {
  const floating = new Map<Element, number>();
  let queue = Promise.resolve();
  const current = () => {
    const el = doc.pictureInPictureElement;
    return el ? (floating.get(el) ?? null) : null;
  };
  return {
    current,
    toggle: (session: number): Promise<void> =>
      (queue = queue.then(async () => {
        const was = current();
        if (doc.pictureInPictureElement) await doc.exitPictureInPicture().catch(() => {});
        if (was !== session) {
          const el = await enter(session).catch(() => null);
          if (el) floating.set(el, session);
        }
        onChange(current());
      })),
    // An element left picture-in-picture (its leavepictureinpicture).
    left: (el: Element) => {
      floating.delete(el);
      onChange(current());
    },
  };
};
