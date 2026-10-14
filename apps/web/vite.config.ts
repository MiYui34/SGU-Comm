import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [svelte()],
  // Workspace source package: do not pre-bundle, or edits are not picked up.
  optimizeDeps: { exclude: ["@kallisti/protocol"] },
});
