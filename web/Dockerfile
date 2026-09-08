# The SvelteKit half, built with adapter-node.
#
# This serves pages only. It never proxies /v1: the API client's base URL is
# relative, so the browser resolves it against whatever origin served the
# page, and Caddy is what puts both halves on that one origin. See the
# Caddyfile -- the same-origin arrangement is what lets the httpOnly refresh
# cookie travel without a cross-site grant.

# ── build ───────────────────────────────────────────────────────────────────
FROM node:22-alpine AS build

WORKDIR /app

# `ci`, not `install`: it installs exactly what the lockfile pins and fails if
# package.json has drifted, rather than quietly resolving something newer than
# what CI tested. Same reason the CI workflow uses it.
COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

# Re-resolve with the dev dependencies dropped. The build needed vite, svelte
# and the adapter; the server that runs afterwards needs none of them, and
# carrying them into the runtime image is most of its size for nothing.
RUN npm prune --omit=dev

# ── runtime ─────────────────────────────────────────────────────────────────
FROM node:22-alpine AS runtime

WORKDIR /app

ENV NODE_ENV=production \
    PORT=3000 \
    HOST=0.0.0.0

# adapter-node emits a self-contained server in build/, but it still resolves
# its runtime dependencies from node_modules, so both come across.
COPY --from=build /app/build ./build
COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/package.json ./package.json

# node:alpine ships an unprivileged `node` user; nothing here needs to write.
USER node

EXPOSE 3000

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:3000/healthz >/dev/null || exit 1

# Not `npm start`: npm forwards SIGTERM to the node process poorly, so a
# container stop becomes a ten-second wait and then a kill. Exec'ing node
# directly makes it PID 1 and lets it shut down when asked.
CMD ["node", "build/index.js"]
