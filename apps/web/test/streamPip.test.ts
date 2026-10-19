import { describe, expect, it } from "vitest";
import { createPip } from "../src/lib/stream/pip";

// A browser's picture-in-picture: one element at most; entering and leaving
// take a moment.
const setup = (fail = new Set<number>()) => {
  const tick = () => new Promise((r) => setTimeout(r, 1));
  const doc = {
    pictureInPictureElement: null as Element | null,
    exitPictureInPicture: async () => {
      await tick();
      const el = doc.pictureInPictureElement;
      doc.pictureInPictureElement = null;
      if (el) pip.left(el);
    },
  };
  const states: (number | null)[] = [];
  const pip = createPip(
    doc,
    async (session) => {
      await tick();
      if (fail.has(session)) return null;
      const el = { session } as unknown as Element;
      doc.pictureInPictureElement = el;
      return el;
    },
    (s) => states.push(s),
  );
  return { doc, pip, states };
};

describe("createPip", () => {
  it("floats a stream and puts it back on a second toggle", async () => {
    const { pip, states } = setup();
    await pip.toggle(1);
    expect(pip.current()).toBe(1);
    await pip.toggle(1);
    expect(pip.current()).toBeNull();
    expect(states.at(-1)).toBeNull();
  });

  it("ends right after two fast clicks", async () => {
    const { pip, states } = setup();
    void pip.toggle(1);
    await pip.toggle(1);
    expect(pip.current()).toBeNull();
    expect(states.at(-1)).toBeNull();
  });

  it("switches to another stream", async () => {
    const { pip, states } = setup();
    void pip.toggle(1);
    await pip.toggle(2);
    expect(pip.current()).toBe(2);
    expect(states.at(-1)).toBe(2);
  });

  it("shows nothing floating when entering failed", async () => {
    const { pip, states } = setup(new Set([2]));
    await pip.toggle(1);
    await pip.toggle(2);
    expect(pip.current()).toBeNull();
    expect(states.at(-1)).toBeNull();
  });

  it("follows the browser when the window is closed there", async () => {
    const { doc, pip, states } = setup();
    await pip.toggle(1);
    const el = doc.pictureInPictureElement!;
    doc.pictureInPictureElement = null;
    pip.left(el);
    expect(states.at(-1)).toBeNull();
  });
});
