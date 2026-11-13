import path from 'node:path'
import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  reactStrictMode: true,
  // The contract package lives in gen/ts, a sibling of the console, so the bundler's root is the repository.
  turbopack: { root: path.join(import.meta.dirname, '..') },
  transpilePackages: ['@tourian/contract'],
  // The development server is reached at the laptop's address on the local network too, by a phone or another browser.
  allowedDevOrigins: ['192.168.*.*', '10.*.*.*', '172.*.*.*', '*.local'],
}

export default nextConfig
