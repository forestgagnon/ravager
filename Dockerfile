FROM golang:1.13 as builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -v -ldflags="-s -w" -o ravager .

FROM alpine:3.10
RUN apk add --update --no-cache ca-certificates
WORKDIR /root/

COPY --from=builder /app/ravager ./
ENTRYPOINT ["/root/ravager"]
