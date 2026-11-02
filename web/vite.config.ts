import { defineConfig } from 'vite'

export default defineConfig({
  // Vercel sets process.env.VERCEL automatically during builds
  base: process.env.VERCEL ? '/' : '/Xochi/',
})
