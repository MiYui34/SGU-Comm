// The colour theme before the app's CSS paints (no dark flash for light
// users). A copy of themeOf in src/lib/settings.ts: keep them the same.
try {
  var theme = (JSON.parse(localStorage.getItem("voiceSettings") || "{}") || {}).theme;
  if (theme === "system") theme = matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  if (theme === "light") {
    document.documentElement.dataset.theme = "light";
    // iOS: dark text on a plain bar (black-translucent has white text).
    document.querySelector('meta[name="apple-mobile-web-app-status-bar-style"]').content = "default";
  }
} catch (e) {}
