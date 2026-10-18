// Download links: the latest release's files, from GitHub (without
// JavaScript, or if GitHub does not answer, they go to the releases page).
// The visitor's system comes first: its card is marked, and the big
// button at the top downloads it.
const REPO = "lambadalambda/kallisti";
const NAMES = { mac: "macOS", windows: "Windows", linux: "Linux" };

const systemOf = () => {
  const p = (navigator.userAgentData?.platform || navigator.platform || navigator.userAgent).toLowerCase();
  if (p.includes("mac")) return "mac";
  if (p.includes("win")) return "windows";
  if (p.includes("linux") && !/android/i.test(navigator.userAgent)) return "linux";
  return null; // phones and others: the browser is the way
};

const system = systemOf();
const card = system && document.querySelector(`.platform[data-os="${system}"]`);
if (card) card.classList.add("yours");

fetch(`https://api.github.com/repos/${REPO}/releases/latest`)
  .then((r) => (r.ok ? r.json() : Promise.reject(new Error(r.statusText))))
  .then((release) => {
    for (const link of document.querySelectorAll("[data-asset]")) {
      const asset = release.assets.find((a) => a.name.endsWith(link.dataset.asset));
      if (asset) link.href = asset.browser_download_url;
    }
    document.getElementById("version").textContent = `Version ${release.tag_name.replace(/^v/, "")}, free and open source.`;
    const hero = document.getElementById("hero-download");
    const main = card?.querySelector(".button");
    if (main) {
      hero.href = main.href;
      hero.textContent = `Download for ${NAMES[system]}`;
    }
  })
  .catch(() => {}); // the links to the releases page stay
