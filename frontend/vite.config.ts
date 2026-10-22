import { statSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";

function mobileDownloads(): Plugin {
  function middleware(directory: string) {
    return (req: { url?: string }, res: import("node:http").ServerResponse, next: () => void) => {
      const pathname = (req.url || "").split("?")[0];
      if (!pathname.startsWith("/downloads/")) return next();
      let available = false;
      try { available = pathname === "/downloads/kfb-invest-android.apk" && statSync(`${directory}/kfb-invest-android.apk`).size > 0; } catch { /* Missing downloads return 404 instead of the SPA. */ }
      if (!available) {
        res.statusCode = 404;
        res.setHeader("Content-Type", "text/plain; charset=utf-8");
        res.end("APK not found");
        return;
      }
      res.setHeader("Content-Type", "application/vnd.android.package-archive");
      res.setHeader("Content-Disposition", 'attachment; filename="kfb-invest-android.apk"');
      res.setHeader("Cache-Control", "no-cache");
      res.setHeader("X-Content-Type-Options", "nosniff");
      next();
    };
  }
  return {
    name: "mobile-downloads",
    configureServer(server) { server.middlewares.use(middleware(fileURLToPath(new URL("./public/downloads", import.meta.url)))); },
    configurePreviewServer(server) { server.middlewares.use(middleware(fileURLToPath(new URL("./dist/downloads", import.meta.url)))); },
  };
}

export default defineConfig({
  plugins: [react(), mobileDownloads()],
  server: {
    port: 15173,
    strictPort: true,
    proxy: {
      "/v1": { target: "http://localhost:18080", ws: true },
      "/health": "http://localhost:18080",
    },
  },
});
