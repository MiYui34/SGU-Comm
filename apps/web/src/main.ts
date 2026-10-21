import { mount } from "svelte";
import App from "./App.svelte";
import { isDesktop, openInBrowser } from "./lib/desktop";
import { openLinksOutside } from "./lib/externalLinks";
import "./app.css";

const app = mount(App, { target: document.getElementById("app")! });

// After mount: Svelte's own document listener (for elements outside the
// app's root) must see clicks first.
if (isDesktop()) openLinksOutside((url) => openInBrowser(url).catch((err) => console.error("link not opened", err)));

// iOS lets the installed app pinch-zoom despite touch-action (app.css):
// its own gesture events stop it there.
const installed = matchMedia("(display-mode: standalone)").matches || (navigator as { standalone?: boolean }).standalone === true;
if (installed) document.addEventListener("gesturestart", (e) => e.preventDefault());

export default app;
