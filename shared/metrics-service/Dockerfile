# Metrics Service Dockerfile (Sidecar for ACVPS Gateway)
FROM node:18-alpine

# Install build dependencies for native modules (better-sqlite3)
RUN apk add --no-cache python3 make g++

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies and rebuild native modules
RUN npm ci --only=production && npm rebuild better-sqlite3

# Copy application code
COPY . .

# Create data directory for SQLite
RUN mkdir -p /app/data && chown -R node:node /app/data

# Use non-root user
USER node

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD node -e "require('http').get('http://localhost:8090/health', (r) => process.exit(r.statusCode === 200 ? 0 : 1))"

# Expose port
EXPOSE 8090

# Start service (initialize DB first)
CMD ["sh", "-c", "node init-db.js && node server.js"]

