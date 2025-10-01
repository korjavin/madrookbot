# Madrook Bot

A Telegram bot for managing group discussions and media suggestions.

## Features

### General
- Text-to-speech using ElevenLabs (mention the bot to have text read aloud)
- GPT-3 integration for smart responses
- **Answer mode**: Send `answer: your question` with bot mention to get GPT responses with custom system prompt
- **Conversation threading**: Reply to bot's answers to continue the conversation with context (maintains up to 5 exchanges in memory)
  - Multiple users can branch conversations from the same bot message
  - Each branch maintains its own conversation history
  - Conversations use the same system prompt from the initial "answer:" query
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
- Mention the bot to have text read aloud using text-to-speech
- `answer: <question>` (with bot mention) - Ask GPT with custom system prompt, then reply to continue the conversation

## Environment Variables

- `BOT_TOKEN` - Telegram bot token
- `GPT_TOKEN` - OpenAI API key (optional)
- `GPT_KEYWORDS` - Keywords to trigger GPT (optional)
- `GPT_SYSTEM_PROMPT` - Custom system prompt for "answer:" mode (optional, defaults to helpful assistant)
- `GPT_MODEL` - OpenAI model to use (e.g., gpt-4, gpt-3.5-turbo)
- `GPT_SYSTEM_MSG` - System message for regular keyword-triggered GPT responses
- `OWNER_ID` - Telegram user ID of the bot owner
- `CLASS_GROUP_ID` - ID of the Telegram group for class discussions
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
