# syntax=docker/dockerfile:1

# ---- build ----
FROM golang:1.25-alpine AS build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server && \
    CGO_ENABLED=0 go build -o /out/test-server ./cmd/test-server

# ---- run ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=build /out/server ./server
COPY --from=build /out/test-server ./test-server
# Static UI (overridable at runtime via WEB_DIR env / compose volume).
COPY web/ ./web/
ENV PORT=8080 \
    WEB_DIR=/app/web \
    TEST_SERVER_PORT=8081 \
    SLOW_DELAY_MS=200
EXPOSE 8080 8081
CMD ["./server"]
