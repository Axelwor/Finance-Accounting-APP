# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api/

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /bin/api /app/api
COPY --from=builder /app/backend/migrations /app/migrations

ENV DATABASE_URL=""
ENV JWT_SECRET=""
ENV HTTP_ADDR=":8080"
ENV STORAGE_ROOT="/data/audit"

EXPOSE 8080
CMD ["/app/api"]
