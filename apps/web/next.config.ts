import type { NextConfig } from "next";

const config: NextConfig = {
  // Standalone tracing uses symlinks; Windows often hits EPERM without Developer Mode.
  // Docker build sets NEXT_STANDALONE_OUTPUT=1 so images still get output: "standalone".
  ...(process.env.NEXT_STANDALONE_OUTPUT === "1"
    ? { output: "standalone" as const }
    : {}),
  reactStrictMode: true,
  images: {
    remotePatterns: [
      { protocol: "https", hostname: "sleepercdn.com" },
      { protocol: "https", hostname: "*.sleepercdn.com" },
    ],
  },
};

export default config;
