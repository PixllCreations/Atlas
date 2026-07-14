import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/apps": "http://localhost:8080",
      "/status": "http://localhost:8080",
      "/healthz": "http://localhost:8080",
      "/webhooks": "http://localhost:8080",
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
