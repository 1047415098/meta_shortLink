import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  base: "/",
  server: {
    host: "0.0.0.0",
    port: 5176,
    strictPort: true,
    proxy: {
      // 开发环境保持与生产相同的真实 API 和上传图片路径。
      "/novel-api": { target: "http://127.0.0.1:8080", changeOrigin: false },
      "/novel-uploads": { target: "http://127.0.0.1:8080", changeOrigin: false },
      // 本地保持与生产相同的签名小说行为，首屏主题本身不会发起额外统计请求。
      "^/novel/[a-zA-Z0-9_-]{3,40}/(view|time-spent|start-reading|reading-time)$": { target: "http://127.0.0.1:8080", changeOrigin: false }
    }
  },
  build: { assetsDir: "novel-assets" }
});
