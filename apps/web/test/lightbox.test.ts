import { describe, expect, it } from "vitest";
import { opensLightbox } from "../src/lib/lightbox";

const click = (o: Partial<MouseEvent> = {}) =>
  ({ button: 0, ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, ...o }) as MouseEvent;

describe("opensLightbox", () => {
  it("opens on a plain left click", () => {
    expect(opensLightbox(click())).toBe(true);
  });

  it("leaves new-tab and other clicks to the browser", () => {
    for (const o of [{ button: 1 }, { ctrlKey: true }, { metaKey: true }, { shiftKey: true }, { altKey: true }]) {
      expect(opensLightbox(click(o))).toBe(false);
    }
  });
});
