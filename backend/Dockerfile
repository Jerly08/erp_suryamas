FROM golang:1.24-alpine AS builder

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
COPY --from=builder /src/api .
# Copy templates and static files if necessary
COPY --from=builder /src/templates ./templates
RUN mkdir -p uploads

EXPOSE 8080

CMD ["./api"]