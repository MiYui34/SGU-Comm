import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig } from "vite";
import { noiseSuppressionAudioWorkletVitePlugin } from "@workadventure/noise-suppression/vite";

export default defineConfig({
  // DeepFilterNet3's worklet and model in development (the build copies them).
  plugins: [svelte(), noiseSuppressionAudioWorkletVitePlugin()],
  // Workspace source package: do not pre-bundle, or edits are not picked up.
  optimizeDeps: { exclude: ["@kallisti/protocol"] },
  build: {
    // The capture worklet must be a file: an inlined data: URL would need
    // data: in the Content Security Policy.
    assetsInlineLimit: (file) => (file.endsWith(".worklet.js") ? false : undefined),
    // The service worker (src/sw.ts) is an entry of its own at /sw.js: its
    // scope is the whole site, and its name must not change.
    rolldownOptions: {
      input: { index: "index.html", sw: "src/sw.ts" },
      output: { entryFileNames: (chunk) => (chunk.name === "sw" ? "sw.js" : "assets/[name]-[hash].js") },
    },
  },
});
