import type { NextConfig } from 'next';

// /api/v1/* is proxied to the Go API by src/middleware.ts (not by a rewrite
// here), so that every call passes the session check and gets the API token.
const nextConfig: NextConfig = {
  eslint: {
    ignoreDuringBuilds: true,
  },
};

export default nextConfig;
