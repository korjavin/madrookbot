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
- `GEMINI_API_KEY` - Google Gemini API key (has free tier limits)
- `GEMINI_IMAGE_MODEL` - Image model to use (default: gemini-2.5-flash-image-preview)

### ElevenLabs TTS (optional)
- `ELEVENLABS_API_KEY` - Your ElevenLabs API key
- `ELEVENLABS_VOICE_NAME` - The name of the ElevenLabs voice to use
- `ELEVENLABS_MODEL_ID` - The ID of the ElevenLabs model to use

### Memory / Context Tool (optional)
- `MEMORY_GROUP_ID` - The ID of the Telegram group where the bot should listen for messages to be stored in the memory.
- `MIN_MESSAGE_LENGTH` - The minimum length of a message to be considered for memory storage (default: 20 characters).
- `MESSAGE_COMBINE_WINDOW` - The time window in seconds within which consecutive messages from the same user are combined into a single entry (default: 90 seconds).
- `QDRANT_HOST` - The hostname or IP address of the Qdrant service (default: `qdrant`).
- `OPENAI_EMBEDDING_MODEL` - The model to use for generating message embeddings (e.g., `text-embedding-3-small`).

## Running

The recommended way to run the application is with `docker-compose`, which manages the bot and the Qdrant vector database.

### With Docker Compose

1.  **Environment File:**
    You will need to create a `.env` file in the project root to store your secrets and configuration. You can do this by copying the provided example file:
    ```bash
    cp .env.example .env
    ```
    Then, edit the `.env` file with your specific credentials (like `BOT_TOKEN`, `GPT_TOKEN`, etc.).

2.  **Start Services:**
    ```bash
    docker-compose up --build -d
    ```
    This command builds the images if they don't exist and starts all services (`madrookbot`, `qdrant`, `tool-api`) in the background.

3.  **View Logs:**
    ```bash
    docker-compose logs -f
    ```

### With Docker (Manual)
If you prefer to run only the bot container without the associated services:
```bash
docker build -t madrookbot .
docker run \
  -e BOT_TOKEN=your_token \
  -e OWNER_ID=your_id \
  -e CLASS_GROUP_ID=group_id \
  # ... other environment variables
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
