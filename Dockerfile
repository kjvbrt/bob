FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -tags prod -ldflags="-s -w" -o fcc-drs ./cmd/fcc-drs

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/fcc-drs ./
COPY --from=builder /app/static ./static
COPY --from=builder /app/templates ./templates
EXPOSE 5050
USER 1001
CMD ["./fcc-drs"]
