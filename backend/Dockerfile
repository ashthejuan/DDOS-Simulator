# syntax=docker/dockerfile:1

# ---- build ----
FROM golang:1.22-alpine AS build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

# ---- run ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/server ./server
# Static UI (overridable at runtime via WEB_DIR env / compose volume).
COPY web/ ./web/
ENV PORT=8080 \
    WEB_DIR=/app/web
EXPOSE 8080
CMD ["./server"]
