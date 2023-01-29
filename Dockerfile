FROM golang:1.19.3
WORKDIR /go/src/app
COPY . .
RUN go build -mod=vendor -o /tmp/madrookbot

FROM debian:testing
RUN apt-get update && apt-get -y install ca-certificates
RUN mkdir /bot
COPY --from=0 /tmp/madrookbot /bot/madrookbot
WORKDIR /bot
ENTRYPOINT ["/bot/madrookbot"]
