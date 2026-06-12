import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  images: {
    // Avatars/crests live in the public Supabase Storage bucket. Allow
    // next/image to optimize them. Covers any *.supabase.co project host.
    remotePatterns: [
      {
        protocol: "https",
        hostname: "*.supabase.co",
        pathname: "/storage/v1/object/public/**",
      },
    ],
  },
};

export default nextConfig;
