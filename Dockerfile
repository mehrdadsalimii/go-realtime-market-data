FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/wsserver ./cmd/wsserver

FROM alpine:3.20
RUN adduser -D appuser
USER appuser

COPY --from=builder /bin/wsserver /wsserver

EXPOSE 5002
ENV ADDR=:5002
CMD ["/wsserver"]
