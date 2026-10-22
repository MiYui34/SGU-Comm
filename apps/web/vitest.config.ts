import { mergeConfig } from "vite";
import { defineConfig } from "vitest/config";
import viteConfig from "./vite.config.ts";

// 测试跑在 jsdom 里，但 vite 默认按 node 条件解析依赖：这样 svelte 会拿到
// 服务端运行时，mount() 直接报 lifecycle_function_unavailable。
// 加上 browser 条件，组件测试才能真的把组件挂到 DOM 上。
export default mergeConfig(
  viteConfig,
  defineConfig({
    resolve: { conditions: ["browser"] },
  }),
);
