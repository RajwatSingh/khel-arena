# Leftovers from the Next.js frontend

These five files were the only things still sitting in `src/` after the move to
Go and Postgres. They were never committed, so deleting them would have lost
them — they are parked here instead.

- `robots.ts`, `sitemap.ts` — ported to `web/src/routes/robots.txt` and
  `web/src/routes/sitemap.xml`. Safe to delete.
- `opengraph-image.tsx`, `twitter-image.tsx`, `og-image.tsx` — the social card,
  drawn with `next/og` (satori). Not ported: it is Next-specific, and the new
  card would want the new palette anyway. Keep until someone redraws it.
