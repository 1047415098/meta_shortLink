import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import { landingBootstrapProxy } from "./dev/bootstrapProxy.js";
export default defineConfig({
  // Development injects the real signed Go bootstrap before Vite adds HMR.
  plugins: [landingBootstrapProxy(), vue()],
  base: "/",
  server: {
    host: "0.0.0.0",
    port: 5174,
    strictPort: true,
    proxy: {
      // Consultation and PageView actions remain owned by the Go service.
      "^/[a-zA-Z0-9_-]{3,40}/(contact|view)$": {
        target: "http://127.0.0.1:8080",
        changeOrigin: false,
      },
    },
  },
  build: { assetsDir: "landing-assets" },
});
