import { mount } from "svelte";
import App from "./App.svelte";
import { isDesktop, openInBrowser } from "./lib/desktop";
import { openLinksOutside } from "./lib/externalLinks";
import "./app.css";

if (isDesktop()) openLinksOutside((url) => openInBrowser(url).catch((err) => console.error("link not opened", err)));

export default mount(App, { target: document.getElementById("app")! });
