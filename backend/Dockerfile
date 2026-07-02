FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git tzdata
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o api cmd/main.go

FROM alpine:latest AS production
RUN apk add --no-cache tzdata ca-certificates
# Set timezone ke Asia/Jakarta
ENV TZ=Asia/Jakarta

WORKDIR /app
RUN addgroup -S app && adduser -S -G app app
COPY --from=builder --chown=app:app /src/api .
# Copy templates and static files if necessary
COPY --from=builder --chown=app:app /src/templates ./templates
# Copy committed OpenAPI spec so Swagger UI can serve the full contract at runtime
COPY --from=builder --chown=app:app /src/openapi ./openapi
# Copy SQL migration files so RunAutoMigrations can find them
COPY --from=builder --chown=app:app /src/migrations ./migrations
RUN mkdir -p uploads && chown -R app:app /app

USER app

EXPOSE 8080

CMD ["./api"]
