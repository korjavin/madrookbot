# Refactoring Plan for Madrookbot

## Overview

This plan outlines beneficial refactoring steps based on code review findings. The goal is to improve testability, maintainability, and code quality while preserving functionality.

## Current Issues Identified

### Critical Issues

| Issue | Location | Impact |
|-------|----------|--------|
| Global variables (9+) | [`chatgpt.go:13`](chatgpt.go:13), [`db.go:10`](db.go:10), [`memory.go:42-45`](memory.go:42-45), [`telegram.go:22`](telegram.go:22), [`news.go:24-29`](news.go:24-29) | Makes testing impossible without mocks |
| Missing nil check | [`chatgpt.go:84`](chatgpt.go:84) | Potential panic |
| Error ignoring | [`telegram.go:234`](telegram.go:234), [`telegram.go:280`](telegram.go:280), [`conversation.go:222`](conversation.go:222) | Silent failures |

### Architecture Issues

1. **Tight Coupling**: Functions directly use global clients instead of depending on interfaces
2. **Hidden Dependencies**: Global state makes it hard to understand dependencies
3. **Mixed Concerns**: Some files handle multiple responsibilities

---

## Phases Selected for Implementation

| Phase | Focus | Status |
|-------|-------|--------|
| 1 | **Interface Extraction** | ⏳ Pending |
| 2 | **Eliminate Globals** | ⏳ Pending |
| 4 | **Code Organization** | ⏳ Pending |
| 5 | **Testing Infrastructure** | ⏳ Pending |

*Phase 3 (Error Handling) will be done incrementally during other phases*

---

## Phase 1: Interface Extraction (High Priority)

### 1.1 Define Core Interfaces

Create a new file `interfaces.go`:

```go
// TelegramBotAPI interface for bot operations
type TelegramBotAPI interface {
    GetMe() (tgbotapi.User, error)
    Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
    GetUpdates(c tgbotapi.UpdateConfig) ([]tgbotapi.Update, error)
    MakeRequest(method string, params tgbotapi.Params) (tgbotapi.APIResponse, error)
    // Add more methods as needed
}

// OpenAIClient interface for AI operations
type OpenAIClient interface {
    CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
    // Add more methods as needed
}

// Database interface for persistence
type Database interface {
    Exec(query string, args ...interface{}) (sql.Result, error)
    Query(query string, args ...interface{}) (*sql.Rows, error)
    QueryRow(query string, args ...interface{}) *sql.Row
    Prepare(query string) (*sql.Stmt, error)
    Begin() (*sql.Tx, error)
    Close() error
}

// QdrantClient interface for vector storage
type QdrantClient interface {
    Scroll(ctx context.Context, req *qdrant.ScrollPoints) (*qdrant.ScrollResponse, error)
    // Add more methods as needed
}
```

### 1.2 Refactor ChatGPT Functions

**File**: [`chatgpt.go`](chatgpt.go)

**Changes:**
- Add `OpenAIClient` interface parameter to [`getGPTAnswerWithSystem()`](chatgpt.go:65) and [`getGPTAnswerWithSystemAndTools()`](chatgpt.go:70)
- Create a wrapper that uses the global client by default for backward compatibility
- Update all callers to pass the client

**Before:**
```go
func getGPTAnswerWithSystem(msg, system string) (string, error) {
    // Uses global client
}
```

**After:**
```go
type GPTClient interface {
    CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

func getGPTAnswerWithSystem(msg, system string, client GPTClient) (string, error) {
    // Uses injected client
}

func getGPTAnswerWithSystemDefault(msg, system string) (string, error) {
    return getGPTAnswerWithSystem(msg, system, openAIClient)
}
```

### 1.3 Refactor Database Access

**File**: [`db.go`](db.go)

**Changes:**
- Wrap the global `db` variable in a package-level getter that returns an interface
- Create a `Database` interface with needed methods
- Update [`conversation.go`](conversation.go) and other files to use the interface

---

## Phase 2: Eliminate Global Variables

### 2.1 Create Service Structs

Group related functionality into services:

```go
// BotService handles Telegram bot operations
type BotService struct {
    bot      TelegramBotAPI
    config   *Config
}

// ConversationService handles conversation caching
type ConversationService struct {
    cache *ConversationCache
    db    Database
}

// NewsService handles news posting
type NewsService struct {
    gpt      OpenAIClient
    qdrant   QdrantClient
    db       Database
    rand     *rand.Rand
}
```

### 2.2 Pass Dependencies Explicitly

**Before (current pattern):**
```go
// Uses global db variable
func initConversationDatabase() error {
    _, err := db.Exec(...) // Direct global access
}
```

**After (injected dependency):**
```go
type DatabaseService interface {
    Exec(query string, args ...interface{}) (sql.Result, error)
}

func initConversationDatabase(db DatabaseService) error {
    _, err := db.Exec(...)
}
```

### 2.3 Update Entry Point

**File**: [`main.go`](main.go)

**Changes:**
- Create service instances in main
- Pass dependencies through the call chain
- Remove global variable initialization from `init()` functions

```go
func main() {
    // Create dependencies
    db := initDatabase()
    gptClient := initGPTClient()
    
    // Create services
    conversationSvc := NewConversationService(db)
    newsSvc := NewNewsService(gptClient, db)
    
    // Start bot with services
    botGo(conversationSvc, newsSvc)
}
```

---

## Phase 3: Error Handling Improvements

### 3.1 Add Nil Checks

**File**: [`chatgpt.go`](chatgpt.go:84)

```go
// Before
resp, err := client.CreateChatCompletion(ctx, req)

// After
if client == nil {
    return "", fmt.Errorf("OpenAI client is nil")
}
resp, err := client.CreateChatCompletion(ctx, req)
if err != nil {
    return "", fmt.Errorf("chat completion failed: %w", err)
}
```

### 3.2 Stop Ignoring Errors

**File**: [`conversation.go`](conversation.go:222)

```go
// Before
rowsAffected, _ := result.RowsAffected()

// After
rowsAffected, err := result.RowsAffected()
if err != nil {
    log.Printf("[WARN] Failed to get rows affected: %v", err)
}
```

### 3.3 Add Structured Logging

Replace simple `log.Printf` with structured logging using a package like `zap` or `zerolog`:

```go
logger.Info("added message to cache",
    zap.Int("messageId", node.MessageID),
    zap.Int("parentId", node.ParentID),
    zap.Int("size", node.ApproxSize),
)
```

---

## Phase 4: Code Organization

### 4.1 Split Large Files

**File**: [`telegram.go`](telegram.go) - Split into:
- `telegram.go` - Main bot loop
- `telegram_handlers.go` - Command handlers
- `telegram_states.go` - State management

**File**: [`news.go`](news.go) - Split into:
- `news.go` - Main news logic
- `news_scheduler.go` - Scheduling logic
- `news_api.go` - External API calls

### 4.2 Extract Constants

Create a `config.go` file:

```go
package main

const (
    ConversationCacheLimit   = 20 * 1024 * 1024 // 20MB
    ConversationRetentionDays = 7
    NewsPostCooldown         = 24 * time.Hour
    NewsQuietDuration        = 3 * time.Hour
    MessageMinLength         = 130
    RSSRandomItemLimit       = 5
)
```

### 4.3 Move Types to Dedicated Files

- Move `ConversationNode` and `ConversationCache` to `conversation_types.go`
- Move `InteractiveNewsSession` to `news_types.go`
- Move `class` struct to `class_types.go`

---

## Phase 5: Testing Infrastructure

### 5.1 Create Mocks

Create `mocks/` directory with generated mocks:

```bash
go install github.com/golang/mock/mockgen@v1.6.0
mockgen -destination=mocks/telegram_mock.go -package=mocks github.com/go-telegram-bot-api/telegram-bot-api/v5 BotAPI
mockgen -destination=mocks/openai_mock.go -package=mocks github.com/sashabaranov/go-openai Client
```

### 5.2 Integration Tests

Create `tests/integration/` directory:

```go
func TestConversationService_EndToEnd(t *testing.T) {
    // Setup test database
    db := setupTestDB()
    
    // Create service with real DB
    svc := NewConversationService(db)
    
    // Test complete workflow
    // ...
}
```

### 5.3 Test Fixtures

Create `tests/fixtures/` for test data:

```go
var testMessages = []ConversationNode{
    {MessageID: 1, Text: "Hello", Role: "user"},
    {MessageID: 2, Text: "Hi there!", Role: "assistant"},
}
```

---

## Priority Order for Refactoring

| Priority | Task | Benefit | Risk |
|----------|------|---------|------|
| 1 | Add nil checks | Crash prevention | Low |
| 2 | Stop ignoring errors | Bug detection | Low |
| 3 | Create interfaces | Testability | Medium |
| 4 | Eliminate globals | Maintainability | Medium |
| 5 | Split large files | Readability | Medium |
| 6 | Structured logging | Debugging | Low |
| 7 | Move types | Organization | Low |

---

## Backward Compatibility

To maintain backward compatibility during refactoring:

1. **Use wrapper functions**: Keep existing global-based functions as wrappers around new injectable versions
2. **Feature flags**: Use environment variables to toggle between old/new implementations
3. **Incremental changes**: Refactor one module at a time, ensuring tests pass before moving on

Example:

```go
// New injectable version
func getGPTAnswerWithSystemInj(msg, system string, client OpenAIClient) (string, error)

// Old wrapper (uses global) - mark as deprecated
func getGPTAnswerWithSystem(msg, system string) (string, error) {
    return getGPTAnswerWithSystemInj(msg, system, globalClient)
}
```

---

## Estimated Impact

- **Testability**: Go from ~0% to 80%+ coverage on core logic
- **Maintainability**: Clear dependencies, easier to understand
- **Reliability**: Fewer panics from nil checks, better error handling
- **Extensibility**: Easy to swap implementations (e.g., different AI providers)

---

## Files to Modify

| File | Changes |
|------|---------|
| `interfaces.go` | New file - interface definitions |
| `chatgpt.go` | Add client parameter, nil checks |
| `db.go` | Add Database interface |
| `memory.go` | Add client parameter |
| `conversation.go` | Use injected dependencies |
| `news.go` | Use injected dependencies |
| `telegram.go` | Use injected dependencies |
| `main.go` | Create and pass dependencies |
| `config.go` | New file - constants |
| `telegram_handlers.go` | New file - split from telegram.go |
| `news_scheduler.go` | New file - split from news.go |
| `conversation_types.go` | New file - type definitions |
| `news_types.go` | New file - type definitions |
