# ─────────────────────────────────────────────────────────────────────────────
# Stage 1 — deps: install only production + build dependencies
# ─────────────────────────────────────────────────────────────────────────────
FROM node:20-alpine AS deps

# libc6-compat is needed for some native npm packages on Alpine
RUN apk add --no-cache libc6-compat

WORKDIR /app

# Copy lockfile and manifests first — maximises Docker layer cache
COPY package.json package-lock.json* ./

# Clean install (respects lockfile, skips devDeps during prod installs later)
RUN npm ci

# ─────────────────────────────────────────────────────────────────────────────
# Stage 2 — builder: compile the Next.js application
# ─────────────────────────────────────────────────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /app

# Re-use node_modules from deps stage
COPY --from=deps /app/node_modules ./node_modules

# Copy entire source tree
COPY . .

# NEXT_PUBLIC_API_BASE_URL must be passed as a build-arg so Next.js can bake
# it into the client bundle at build time (NEXT_PUBLIC_* vars are inlined).
# Example:
#   docker build --build-arg NEXT_PUBLIC_API_BASE_URL=https://api.example.com .
ARG NEXT_PUBLIC_API_BASE_URL=""
ENV NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL

# Disable Next.js telemetry during build
ENV NEXT_TELEMETRY_DISABLED=1

# Build the application — outputs to .next/
RUN npm run build

# ─────────────────────────────────────────────────────────────────────────────
# Stage 3 — runner: minimal production image
# ─────────────────────────────────────────────────────────────────────────────
FROM node:20-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# Create a non-root user for security
RUN addgroup --system --gid 1001 nodejs \
 && adduser  --system --uid 1001 nextjs

# Copy only what Next.js needs to run (standalone output is not enabled,
# so we copy the full .next build + public + node_modules)
COPY --from=builder /app/public       ./public
COPY --from=builder /app/.next        ./.next
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./package.json
COPY --from=builder /app/next.config.js ./next.config.js

# Set correct file ownership
RUN chown -R nextjs:nodejs /app

USER nextjs

EXPOSE 3000

# PORT env var lets you override via docker run -e PORT=3001 if needed
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

# `next start` serves the pre-built production bundle
CMD ["node_modules/.bin/next", "start", "-p", "3000", "-H", "0.0.0.0"]
