FROM golang:1.24
WORKDIR /go/src/app
COPY . .
RUN go build -mod=vendor -o /tmp/madrookbot .
RUN go build -mod=vendor -o /tmp/tool-api ./cmd/tool-api

FROM debian:testing
RUN apt-get update && apt-get -y install ca-certificates
RUN mkdir /bot
COPY --from=0 /tmp/madrookbot /bot/madrookbot
COPY --from=0 /tmp/tool-api /bot/tool-api
WORKDIR /bot
# Environment variables:
# BOT_TOKEN - Telegram bot token
# GPT_TOKEN - OpenAI API key (optional)
# GPT_KEYWORDS - Keywords to trigger GPT (optional)
# OWNER_ID - Telegram user ID of the bot owner
# Default command (can be overridden in docker-compose.yml)
CMD ["/bot/madrookbot"]