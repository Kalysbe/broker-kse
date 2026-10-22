FROM golang:1.24-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/seed ./cmd/seed && \
    CGO_ENABLED=0 GOOS=linux go build -o /out/orderload ./cmd/orderload

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
RUN mkdir -p /data/objects && chown nobody:nobody /data/objects
WORKDIR /app
COPY --from=build /out/api /out/worker /out/seed /out/orderload /app/
COPY migrations /app/migrations
ENV APP_ENV=production
EXPOSE 8080 9090
USER nobody
CMD ["/app/api"]
