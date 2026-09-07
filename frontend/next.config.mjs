import { fileURLToPath } from 'node:url'
import { dirname } from 'node:path'

/** @type {import('next').NextConfig} */
const nextConfig = {
  allowedDevOrigins: ['muzimi.xyz'],
  turbopack: {
    root: dirname(fileURLToPath(import.meta.url)),
  },
  typescript: {
    ignoreBuildErrors: true,
  },
  // 开发环境默认使用 Webpack，以便通过 watchOptions 排除编辑器和构建产物目录。
  // Next.js 16 的 `watchOptions` 不支持 ignored；该规则不能直接套用到 Turbopack。
  // 如需恢复 Turbopack，可运行 `npm run dev:turbopack`。
  webpack(config, { dev }) {
    if (dev) {
      config.watchOptions = {
        ...config.watchOptions,
        ignored: [
          '**/node_modules/**',
          '**/.next/**',
          '**/.trae/**',
          '**/dist/**',
          '**/coverage/**',
        ],
      }
    }
    return config
  },
  images: {
    unoptimized: true,
  },
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: 'http://localhost:8080/:path*',
      },
    ]
  },
}

export default nextConfig
