# syntax=docker/dockerfile:1.6

FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/todo-api .

# Prepare a writable /data owned by the distroless nonroot user (65532).
FROM alpine:3.20 AS prep
RUN mkdir -p /data && chown -R 65532:65532 /data

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/todo-api /app/todo-api
COPY --from=prep --chown=65532:65532 /data /data
ENV TODO_DATA_FILE=/data/todos.json \
    PORT=9090
EXPOSE 9090
USER nonroot:nonroot
ENTRYPOINT ["/app/todo-api"]
