import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig } from "vite";
import { noiseSuppressionAudioWorkletVitePlugin } from "@workadventure/noise-suppression/vite";

export default defineConfig({
  // DeepFilterNet3's worklet and model in development (the build copies them).
  plugins: [svelte(), noiseSuppressionAudioWorkletVitePlugin()],
  // Workspace source package: do not pre-bundle, or edits are not picked up.
  optimizeDeps: { exclude: ["@kallisti/protocol"] },
  // The capture worklet must be a file: an inlined data: URL would need
  // data: in the Content Security Policy.
  build: { assetsInlineLimit: (file) => (file.endsWith(".worklet.js") ? false : undefined) },
});
