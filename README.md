# Madrook Bot

A Telegram bot for managing group discussions and media suggestions.

## Features

### General
- Text-to-speech using AWS Polly (mention the bot to have text read aloud)
- Voice selection with `/setvoice` command
- GPT-3 integration for smart responses
- Scheduled class management

### Media Management
The bot helps manage a weekly discussion group by:

1. **Collecting Media Suggestions**
   - Automatically detects when users suggest media (articles, videos, podcasts, etc.)
   - Recognizes phrases like "I suggest this", "what about", "consider", etc. followed by a URL
   - Maintains a database of suggestions with metadata

2. **Weekly Schedule**
   - **Monday 15:00 (Berlin)**: Reminder that media collection is active, shows current list
   - **Wednesday 12:00 (Berlin)**: Presents the list for selection, group owner selects by sending a number
   - **Sunday 17:00 (Berlin)**: Reminder about the upcoming discussion in 1 hour

3. **Media Lifecycle**
   - Unselected media is carried over to next week
   - Media suggestions are removed after 6 weeks if not selected
   - Selected media is announced. The original implementation intended to pin this message, but this is currently disabled due to limitations in the Telegram library being used.

## Commands

- `/help` - Shows help message
- `/setvoice` - Set your preferred voice for text-to-speech
- `/media` or `/list` - Shows the current list of media suggestions
- `/del [number]` - Delete a suggestion (owner can delete any, users can delete their own)
- `/idiom [term]` - Shows the definition of an idiom
- `/cancel` - Cancel current command

## Environment Variables

- `BOT_TOKEN` - Telegram bot token
- `GPT_TOKEN` - OpenAI API key (optional)
- `GPT_KEYWORDS` - Keywords to trigger GPT (optional)
- `OWNER_ID` - Telegram user ID of the bot owner
- `CLASS_GROUP_ID` - ID of the Telegram group for class discussions
- `AWS_ACCESS_KEY_ID` - AWS access key ID for Polly
- `AWS_SECRET_ACCESS_KEY` - AWS secret access key for Polly
- `AWS_REGION` - AWS region for Polly (e.g., `us-west-2`)

## Running

### With Docker
```bash
docker build -t madrookbot .
docker run \
  -e BOT_TOKEN=your_token \
  -e OWNER_ID=your_id \
  -e CLASS_GROUP_ID=group_id \
  -e AWS_ACCESS_KEY_ID=your_aws_key \
  -e AWS_SECRET_ACCESS_KEY=your_aws_secret \
  -e AWS_REGION=your_aws_region \
  madrookbot
```

### Without Docker
```bash
go build -mod=vendor -o madrookbot
./madrookbot
```

## Development

### Build & Test
- Build: `go build -mod=vendor -o madrookbot`
- Run all tests: `go test -v ./...`
- Run a single test: `go test -v -run=TestMyFunction`

### Code Style
- Standard Go formatting (`gofmt`)
- Group standard library imports first, then third-party packages.
- Use CamelCase for exported names and lowerCamelCase for unexported names.
- Check errors immediately and use an early return pattern.
- Add comments for all exported functions and types.

### Project Structure
- The project follows a single-package architecture.
- Dependencies are vendored.
- Configuration is managed through environment variables.
