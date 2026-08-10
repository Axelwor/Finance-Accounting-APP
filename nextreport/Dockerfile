FROM node:20-alpine

WORKDIR /app

COPY server.js .
COPY templates ./templates

ENV NODE_ENV=production
ENV PORT=3100

EXPOSE 3100

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -qO- http://127.0.0.1:3100/health || exit 1

USER node

CMD ["node", "server.js"]
