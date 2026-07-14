# syntax=docker/dockerfile:1

# --- Frontend ---
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- API binary ---
FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/atlas-api ./cmd/api

# --- Runtime ---
FROM alpine:3.21
RUN apk add --no-cache ca-certificates postgresql-client wget \
	&& adduser -D -u 1000 atlas
WORKDIR /home/atlas

COPY --from=build /out/atlas-api /usr/local/bin/atlas-api
COPY store/migrations /migrations
COPY hack/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

USER atlas
EXPOSE 8080
ENV ATLAS_PORT=8080
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["atlas-api"]
