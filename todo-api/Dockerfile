# syntax=docker/dockerfile:1.6

FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/todo-api .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S -G app app && \
    mkdir -p /data && chown -R app:app /data
WORKDIR /app
COPY --from=builder /out/todo-api /app/todo-api
ENV TODO_DATA_FILE=/data/todos.json \
    PORT=9090
VOLUME ["/data"]
EXPOSE 9090
USER app:app
ENTRYPOINT ["/app/todo-api"]
