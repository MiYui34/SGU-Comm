import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [svelte()],
  // Workspace source package: do not pre-bundle, or edits are not picked up.
  optimizeDeps: { exclude: ["@kallisti/protocol"] },
  // The capture worklet must be a file: an inlined data: URL would need
  // data: in the Content Security Policy.
  build: { assetsInlineLimit: (file) => (file.endsWith(".worklet.js") ? false : undefined) },
});
