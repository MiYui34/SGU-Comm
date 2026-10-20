import { mount } from "svelte";
import App from "./App.svelte";
import { isDesktop, openInBrowser } from "./lib/desktop";
import { openLinksOutside } from "./lib/externalLinks";
import "./app.css";

const app = mount(App, { target: document.getElementById("app")! });

// After mount: Svelte's own document listener (for elements outside the
// app's root) must see clicks first.
if (isDesktop()) openLinksOutside((url) => openInBrowser(url).catch((err) => console.error("link not opened", err)));

export default app;
