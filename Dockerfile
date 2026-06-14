# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /quiz ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /quiz /quiz
EXPOSE 8080
USER nonroot:nonroot
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/quiz", "-health"]
ENTRYPOINT ["/quiz"]
