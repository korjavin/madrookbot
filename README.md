# Madrook Bot

A Telegram bot for managing group discussions and media suggestions.

## Features

### General
- **GPT Integration**: Mention the bot to get AI-powered text responses with conversation threading
  - Reply to bot's answers to continue conversations with context (5 exchanges history)
  - Multiple users can branch conversations from the same message
  - Database-backed conversation persistence with 7-day retention
- **Image Generation**: Use `image: <prompt>` to generate images via Google Gemini (free tier available)
  - Case-insensitive prefix
  - Supports Imagen 3 (default) or configurable models
  - Generates and sends images directly in chat
- **Text-to-Speech**: Use `read: <text>` to generate audio via ElevenLabs
- **Activity Statistics**: `/stat` command shows group activity charts (admins only)
  - Tracks message activity in hourly buckets
  - Generates visual charts with top 10 contributors
  - GPT-powered analysis of activity patterns
  - 6-month data retention
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
- `/media` or `/list` - Shows the current list of media suggestions
- `/del [number]` - Delete a suggestion (owner can delete any, users can delete their own)
- `/idiom <term>` - Shows the definition of an idiom from idioms.thefreedictionary.com
- `/stat` - Show group activity statistics (admins only, 1/hour rate limit)
- Mention the bot (`@bot_name <question>`) - Ask GPT, reply to continue conversation
- `image: <prompt>` - Generate images using Google Gemini AI
- `read: <text>` - Convert text to speech using ElevenLabs

## Environment Variables

### Required
- `BOT_TOKEN` - Telegram bot token
- `OWNER_ID` - Telegram user ID of the bot owner
- `CLASS_GROUP_ID` - ID of the Telegram group for class discussions

### OpenAI/GPT (optional)
- `GPT_TOKEN` - OpenAI API key
- `GPT_MODEL` - OpenAI model to use (e.g., gpt-4, gpt-3.5-turbo)
- `GPT_SYSTEM_PROMPT` - Custom system prompt for conversations (defaults to helpful assistant)
- `OPENAI_TEMPERATURE` - Temperature for GPT responses (default: 1.0)
- `OPENAI_URL` - OpenAI API base URL (default: https://api.openai.com/v1)

### Google Gemini Image Generation (optional)
- `GEMINI_API_KEY` - Google Gemini API key (free tier available)
- `GEMINI_IMAGE_MODEL` - Image model to use (default: imagen-3.0-generate-001)

### ElevenLabs TTS (optional)
- `ELEVENLABS_API_KEY` - Your ElevenLabs API key
- `ELEVENLABS_VOICE_NAME` - The name of the ElevenLabs voice to use
- `ELEVENLABS_MODEL_ID` - The ID of the ElevenLabs model to use

## Running

### With Docker
```bash
docker build -t madrookbot .
docker run \
  -e BOT_TOKEN=your_token \
  -e OWNER_ID=your_id \
  -e CLASS_GROUP_ID=group_id \
  -e ELEVENLABS_API_KEY=your_elevenlabs_key \
  -e ELEVENLABS_VOICE_NAME=your_elevenlabs_voice_name \
  -e ELEVENLABS_MODEL_ID=your_elevenlabs_model_id \
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
