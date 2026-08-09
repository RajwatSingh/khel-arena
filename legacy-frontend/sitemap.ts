import type { MetadataRoute } from "next";

const siteUrl = process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000";

// Public, crawlable routes. (Personal/auth pages are excluded — see robots.ts.)
// Per-arena pages could be appended dynamically once a DB is wired in.
export default function sitemap(): MetadataRoute.Sitemap {
  const now = new Date();
  const paths = ["", "/book", "/tournaments", "/teams", "/community"];

  return paths.map((path) => ({
    url: `${siteUrl}${path}`,
    lastModified: now,
    changeFrequency: "daily",
    priority: path === "" ? 1 : 0.7,
  }));
}
