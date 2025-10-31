FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY main.go .
RUN go build -o test-workload main.go

FROM alpine:latest
COPY --from=builder /app/test-workload /test-workload
CMD ["/test-workload"]
