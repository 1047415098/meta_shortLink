import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  base: "/",
  server: {
    host: "0.0.0.0",
    port: 5177,
    strictPort: true,
    proxy: {
      // Development uses the same signed routes as production.
      "^/cover/[a-zA-Z0-9_-]{3,40}/(view|time-spent|visible-time)$": { target: "http://127.0.0.1:8080", changeOrigin: false },
    },
  },
  build: { assetsDir: "cover-assets" },
});
