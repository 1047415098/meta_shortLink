import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
export default defineConfig({
  plugins: [vue()],
  server: {
    // Stable ports keep local bookmarks and HMR WebSocket discovery reliable.
    host: "0.0.0.0",
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": "http://127.0.0.1:8080",
      // 开发后台也从 Gin 读取持久化封面，确保编辑页能正常回显。
      "/novel-uploads": "http://127.0.0.1:8080",
      "/audio-novel-uploads": "http://127.0.0.1:8080",
    },
  },
  build: {
    assetsDir: "admin-assets",
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (
            id.includes("/node_modules/echarts/") ||
            id.includes("/node_modules/zrender/")
          )
            return "charts";
          if (id.includes("/node_modules/element-plus/")) return "ui";
          if (
            id.includes("/node_modules/@vue/") ||
            id.includes("/node_modules/vue/")
          )
            return "vue";
        },
      },
    },
  },
});
