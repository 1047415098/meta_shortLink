FROM node:22-alpine AS frontend
WORKDIR /web
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM node:22-alpine AS landing
WORKDIR /landing
COPY landing/package*.json ./
RUN npm ci
COPY landing/ ./
RUN npm run build

FROM node:22-alpine AS audio-novel
WORKDIR /audio-novel
COPY audio-novel/package*.json ./
RUN npm ci
COPY audio-novel/ ./
RUN npm run build

FROM golang:1.27.1-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /server ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && addgroup -S app && adduser -S app -G app \
    && mkdir -p /app/data/audio-novel-uploads && chown -R app:app /app/data
WORKDIR /app
COPY --from=backend /server /app/server
COPY --from=frontend /web/dist /app/admin
COPY --from=landing /landing/dist /app/landing
COPY --from=audio-novel /audio-novel/dist /app/audio-novel
USER app
ENV LISTEN_ADDR=0.0.0.0:8080 FRONTEND_DIR=/app/admin LANDING_DIR=/app/landing AUDIO_NOVEL_DIR=/app/audio-novel AUDIO_NOVEL_UPLOAD_DIR=/app/data/audio-novel-uploads
EXPOSE 8080
CMD ["/app/server"]
