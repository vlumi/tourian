import path from 'node:path'
import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  reactStrictMode: true,
  // The contract package lives in gen/ts, a sibling of the console, so the bundler's root is the repository.
  turbopack: { root: path.join(import.meta.dirname, '..') },
  transpilePackages: ['@tourian/contract'],
}

export default nextConfig
