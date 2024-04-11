FROM golang:1 as builder

WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o app-msg-broker

FROM alpine

WORKDIR /app
COPY --from=builder /app/app-msg-broker /app/

EXPOSE 8080
CMD ["/app/app-msg-broker"]
