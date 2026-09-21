import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  base: "/",
  server: {
    host: "0.0.0.0",
    port: 5175,
    strictPort: true,
    proxy: {
      // 公开内容由同一个 Gin 服务提供，开发时也保持真实数据流。
      "/audio-novel-api": { target: "http://127.0.0.1:8080", changeOrigin: false },
      "/audio-novel-uploads": { target: "http://127.0.0.1:8080", changeOrigin: false },
      // 开发环境的统计动作仍由本地 Gin 服务处理。
      "^/audio-novel/[a-zA-Z0-9_-]{3,40}/(contact|view|time-spent)$": {
        target: "http://127.0.0.1:8080",
        changeOrigin: false
      }
    }
  },
  build: { assetsDir: "audio-novel-assets" }
});
