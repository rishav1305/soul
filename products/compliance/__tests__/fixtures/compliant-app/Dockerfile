FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
RUN addgroup -g 1001 appuser && adduser -D -u 1001 -G appuser appuser
USER appuser
HEALTHCHECK --interval=30s CMD wget -qO- http://localhost:3000/health || exit 1
EXPOSE 3000
CMD ["node", "server.js"]
