import { describe, expect, it } from "vitest";
import { isNewVersion, mainScript } from "../src/lib/update";

const page = (src: string) => `<!doctype html><html><head><link rel="icon" href="/x.png"><script type="module" crossorigin src="${src}"></script></head></html>`;

describe("update notice", () => {
  it("finds the main script of a page", () => {
    expect(mainScript(page("/assets/index-AAA.js"))).toBe("/assets/index-AAA.js");
    expect(mainScript("<html></html>")).toBeNull();
  });
  it("sees a new version when the server's main script differs", () => {
    expect(isNewVersion("/assets/index-AAA.js", page("/assets/index-BBB.js"))).toBe(true);
    expect(isNewVersion("/assets/index-AAA.js", page("/assets/index-AAA.js"))).toBe(false);
  });
  it("stays quiet when it cannot tell (development, a broken page)", () => {
    expect(isNewVersion("/src/main.ts", page("/src/main.ts"))).toBe(false);
    expect(isNewVersion("/src/main.ts", page("/assets/index-BBB.js"))).toBe(false);
    expect(isNewVersion("/assets/index-AAA.js", "<html>error</html>")).toBe(false);
    expect(isNewVersion(null, page("/assets/index-BBB.js"))).toBe(false);
  });
});
