import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Genera .next/standalone: un server.js con solo las dependencias que usa,
  // para que la imagen de Docker no necesite node_modules completo.
  output: "standalone",
  poweredByHeader: false,
  reactStrictMode: true,
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Referrer-Policy", value: "same-origin" },
        ],
      },
    ];
  },
};

export default nextConfig;
