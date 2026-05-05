import type { NextConfig } from "next";

const config: NextConfig = {
  output: "standalone",
  reactStrictMode: true,
  images: {
    remotePatterns: [
      { protocol: "https", hostname: "sleepercdn.com" },
      { protocol: "https", hostname: "*.sleepercdn.com" },
    ],
  },
};

export default config;
