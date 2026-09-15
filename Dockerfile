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

FROM golang:1.27.1-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /server ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=backend /server /app/server
COPY --from=frontend /web/dist /app/admin
COPY --from=landing /landing/dist /app/landing
USER app
ENV LISTEN_ADDR=0.0.0.0:8080 FRONTEND_DIR=/app/admin LANDING_DIR=/app/landing
EXPOSE 8080
CMD ["/app/server"]
