# syntax=docker/dockerfile:1.7

# ---------- dev ----------
FROM node:22-alpine AS dev
RUN apk add --no-cache libc6-compat && corepack enable
WORKDIR /app
COPY package.json pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile=false
COPY . .
EXPOSE 3000
CMD ["pnpm", "dev"]

# ---------- deps ----------
FROM node:22-alpine AS deps
RUN apk add --no-cache libc6-compat && corepack enable
WORKDIR /app
COPY package.json pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile=false --prod=false

# ---------- build ----------
FROM node:22-alpine AS build
RUN apk add --no-cache libc6-compat && corepack enable
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_WS_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_WS_URL=$NEXT_PUBLIC_WS_URL
RUN pnpm build

# ---------- runtime ----------
FROM node:22-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production
RUN addgroup -S nodejs && adduser -S nextjs -G nodejs
COPY --from=build --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=build --chown=nextjs:nodejs /app/.next/static ./.next/static
COPY --from=build --chown=nextjs:nodejs /app/public ./public
USER nextjs
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD wget -qO- http://localhost:3000/ || exit 1
CMD ["node", "server.js"]
