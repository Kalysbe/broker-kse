import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 15173,
    strictPort: true,
    proxy: {
      "/v1": "http://localhost:18080",
      "/health": "http://localhost:18080",
    },
  },
});
