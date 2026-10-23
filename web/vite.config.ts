import { defineConfig } from "vite"
import { devtools } from "@tanstack/devtools-vite"
import { tanstackStart } from "@tanstack/react-start/plugin/vite"
import viteReact from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"

const config = defineConfig({
  resolve: { tsconfigPaths: true },
  plugins: [devtools(), tailwindcss(), tanstackStart(), viteReact()],
  server: {
    proxy: {
      // Keep the browser on same-origin `/api` while developing the Go API
      // locally on its conventional port.
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
})

export default config
