// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { externalUrl, openLinksOutside } from "../src/lib/externalLinks";

const app = "tauri://localhost";

describe("externalUrl", () => {
  it("opens http(s) links to other origins outside", () => {
    expect(externalUrl("https://example.org/a?b#c", app)).toBe("https://example.org/a?b#c");
    expect(externalUrl("http://example.org", app)).toBe("http://example.org/");
    expect(externalUrl("https://kallisti.example/files/1/clip.mp4", app)).toBe("https://kallisti.example/files/1/clip.mp4");
  });

  it("keeps links to the app itself and other schemes in the app", () => {
    expect(externalUrl("#", app)).toBeNull();
    expect(externalUrl("/channels/1/2", "http://localhost:5173")).toBeNull();
    expect(externalUrl("http://localhost:5173/x", "http://localhost:5173")).toBeNull();
    expect(externalUrl("mailto:a@example.org", app)).toBeNull();
    expect(externalUrl("javascript:alert(1)", app)).toBeNull();
    expect(externalUrl("blob:tauri://localhost/123", app)).toBeNull();
    expect(externalUrl("http://[broken", app)).toBeNull();
  });
});

describe("openLinksOutside", () => {
  let stop = () => {};
  afterEach(() => {
    stop();
    document.body.innerHTML = "";
  });

  const setUp = () => {
    const open = vi.fn();
    stop = openLinksOutside(open);
    document.body.innerHTML = `<a id="ext" href="https://example.org/" target="_blank"><span>x</span></a><a id="own" href="/x">y</a>`;
    return open;
  };
  const click = (id: string, init: MouseEventInit = {}, type = "click") => {
    const e = new MouseEvent(type, { bubbles: true, cancelable: true, ...init });
    (document.querySelector(`#${id} span`) ?? document.querySelector(`#${id}`))!.dispatchEvent(e);
    return e;
  };

  it("opens external links (also middle clicks) and stops the web view's own handling", () => {
    const open = setUp();
    expect(click("ext").defaultPrevented).toBe(true);
    expect(click("ext", { button: 1 }, "auxclick").defaultPrevented).toBe(true);
    expect(open.mock.calls).toEqual([["https://example.org/"], ["https://example.org/"]]);
  });

  it("leaves own links, right clicks and clicks the app handled alone", () => {
    const open = setUp();
    expect(click("own").defaultPrevented).toBe(false);
    click("ext", { button: 2 }, "auxclick");
    document.querySelector("#ext")!.addEventListener("click", (e) => e.preventDefault());
    click("ext");
    expect(open).not.toHaveBeenCalled();
  });

  it("stops when asked", () => {
    const open = setUp();
    stop();
    click("ext");
    expect(open).not.toHaveBeenCalled();
  });
});
