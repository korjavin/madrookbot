FROM debian:testing
RUN apt-get update && apt-get -y install ca-certificates
RUN mkdir /bot
ADD madrookbot /bot/madrookbot

WORKDIR /bot
ENTRYPOINT ["/bot/madrookbot"]
