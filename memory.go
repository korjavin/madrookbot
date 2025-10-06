package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// QdrantPoint represents the structure of a document stored in Qdrant.
type QdrantPoint struct {
	ID           string    `json:"id"`
	MessageID    int64     `json:"message_id"`
	UserID       string    `json:"user_id"`
	Text         string    `json:"text"`
	TimestampUTC time.Time `json:"timestamp_utc"`
	TelegramLink string    `json:"telegram_link"`
}

// recentMessage holds information about a user's last message to enable combining.
type recentMessage struct {
	qdrantID     string
	messageID    int64 // The ID of the *first* message in the series
	text         string
	timestamp    time.Time
	telegramLink string
}

var (
	qdrantClient      qdrant.PointsClient
	openaiClient      *openai.Client
	recentMessages    = make(map[string]recentMessage)
	recentMessagesMux sync.Mutex
)

const (
	qdrantCollectionName = "telegram_messages"
	embeddingSize      = 1536 // For text-embedding-3-small
)

// initQdrantClient initializes the connection to the Qdrant vector database.
func initQdrantClient() error {
	qdrantHost := os.Getenv("QDRANT_HOST")
	if qdrantHost == "" {
		qdrantHost = "localhost"
	}
	qdrantPort := 6334 // Default Qdrant gRPC port

	addr := fmt.Sprintf("%s:%d", qdrantHost, qdrantPort)
	log.Printf("[INFO] Connecting to Qdrant at %s", addr)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("could not connect to Qdrant: %w", err)
	}

	qdrantClient = qdrant.NewPointsClient(conn)

	// Ensure the collection exists
	collectionsClient := qdrant.NewCollectionsClient(conn)
	_, err = collectionsClient.Get(context.Background(), &qdrant.GetCollectionInfoRequest{
		CollectionName: qdrantCollectionName,
	})

	if err != nil {
		// Collection does not exist, so create it
		log.Printf("[INFO] Collection '%s' not found, creating it.", qdrantCollectionName)
		_, createErr := collectionsClient.Create(context.Background(), &qdrant.CreateCollection{
			CollectionName: qdrantCollectionName,
			VectorsConfig: &qdrant.VectorsConfig{
				Config: &qdrant.VectorsConfig_Params{
					Params: &qdrant.VectorParams{
						Size:     embeddingSize,
						Distance: qdrant.Distance_Cosine,
					},
				},
			},
		})
		if createErr != nil {
			return fmt.Errorf("could not create Qdrant collection: %w", createErr)
		}
		log.Printf("[INFO] Collection '%s' created successfully.", qdrantCollectionName)
	} else {
		log.Printf("[INFO] Qdrant collection '%s' already exists.", qdrantCollectionName)
	}

	return nil
}

// generateEmbedding generates a vector embedding for the given text using OpenAI.
func generateEmbedding(text string) ([]float32, error) {
	if openaiClient == nil {
		openaiClient = openai.NewClient(os.Getenv("GPT_TOKEN"))
	}
	model := os.Getenv("OPENAI_EMBEDDING_MODEL")
	if model == "" {
		model = "text-embedding-3-small" // Default model
	}

	req := openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.EmbeddingModel(model),
	}

	resp, err := openaiClient.CreateEmbeddings(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data returned")
	}

	return resp.Data[0].Embedding, nil
}

// upsertMessageToQdrant stores or updates a message and its embedding in Qdrant.
func upsertMessageToQdrant(point QdrantPoint, vector []float32) error {
	payload := map[string]*qdrant.Value{
		"message_id":    {Kind: &qdrant.Value_IntegerValue{IntegerValue: point.MessageID}},
		"user_id":       {Kind: &qdrant.Value_StringValue{StringValue: point.UserID}},
		"text":          {Kind: &qdrant.Value_StringValue{StringValue: point.Text}},
		"timestamp_utc": {Kind: &qdrant.Value_StringValue{StringValue: point.TimestampUTC.Format(time.RFC3339)}},
		"telegram_link": {Kind: &qdrant.Value_StringValue{StringValue: point.TelegramLink}},
	}

	wait := true
	_, err := qdrantClient.Upsert(context.Background(), &qdrant.UpsertPoints{
		CollectionName: qdrantCollectionName,
		Points: []*qdrant.PointStruct{
			{
				Id:      &qdrant.PointId{PointIdOptions: &qdrant.PointId_Uuid{Uuid: point.ID}},
				Vectors: &qdrant.Vectors{VectorsOptions: &qdrant.Vectors_Vector{Vector: &qdrant.Vector{Data: vector}}},
				Payload: payload,
			},
		},
		Wait: &wait,
	})

	if err != nil {
		return fmt.Errorf("failed to upsert point to Qdrant: %w", err)
	}

	log.Printf("[INFO] Successfully upserted message with ID %s to Qdrant.", point.ID)
	return nil
}

// processMessageForMemory is the main function to handle incoming messages for the memory feature.
func processMessageForMemory(message *tgbotapi.Message) {
	// Get config from environment variables
	minLenStr := os.Getenv("MIN_MESSAGE_LENGTH")
	minLen, err := strconv.Atoi(minLenStr)
	if err != nil {
		minLen = 20 // Default value
	}

	combineWindowStr := os.Getenv("MESSAGE_COMBINE_WINDOW")
	combineWindow, err := strconv.Atoi(combineWindowStr)
	if err != nil {
		combineWindow = 90 // Default value
	}

	// 1. Filter: Ignore short messages
	if len(message.Text) < minLen {
		log.Printf("[MEMORY] Ignoring short message from %s (length %d)", message.From.UserName, len(message.Text))
		return
	}

	userID := hashUserID(message.From.UserName)
	now := time.Now()

	recentMessagesMux.Lock()
	defer recentMessagesMux.Unlock()

	var pointToUpsert QdrantPoint
	var textToEmbed string

	// 2. Combine: Check for recent messages from the same user
	if lastMsg, exists := recentMessages[userID]; exists && now.Sub(lastMsg.timestamp).Seconds() < float64(combineWindow) {
		// Combine with the previous message
		log.Printf("[MEMORY] Combining message for user %s", message.From.UserName)
		textToEmbed = lastMsg.text + "\n" + message.Text

		pointToUpsert = QdrantPoint{
			ID:           lastMsg.qdrantID,
			MessageID:    lastMsg.messageID, // Use first message's ID
			UserID:       userID,
			Text:         textToEmbed,
			TimestampUTC: now.UTC(),
			TelegramLink: lastMsg.telegramLink, // Use first message's link
		}

		// Update cache with new combined text and timestamp
		lastMsg.text = textToEmbed
		lastMsg.timestamp = now
		recentMessages[userID] = lastMsg

	} else {
		// Create a new message entry
		log.Printf("[MEMORY] Creating new message entry for user %s", message.From.UserName)
		textToEmbed = message.Text
		qdrantID := uuid.New().String()

		// Generate the correct Telegram link format
		chatIDStr := strconv.FormatInt(message.Chat.ID, 10)
		if strings.HasPrefix(chatIDStr, "-100") {
			chatIDStr = strings.TrimPrefix(chatIDStr, "-100")
		} else if strings.HasPrefix(chatIDStr, "-") {
			chatIDStr = strings.TrimPrefix(chatIDStr, "-")
		}
		link := fmt.Sprintf("https://t.me/c/%s/%d", chatIDStr, message.MessageID)

		pointToUpsert = QdrantPoint{
			ID:           qdrantID,
			MessageID:    int64(message.MessageID),
			UserID:       userID,
			Text:         textToEmbed,
			TimestampUTC: now.UTC(),
			TelegramLink: link,
		}

		// Store in cache for potential future combinations
		recentMessages[userID] = recentMessage{
			qdrantID:     qdrantID,
			messageID:    int64(message.MessageID),
			text:         textToEmbed,
			timestamp:    now,
			telegramLink: link,
		}
	}

	// 3. Generate Embedding
	vector, err := generateEmbedding(textToEmbed)
	if err != nil {
		log.Printf("[ERROR] [MEMORY] Failed to generate embedding for user %s: %v", message.From.UserName, err)
		return
	}

	// 4. Upsert to Qdrant
	err = upsertMessageToQdrant(pointToUpsert, vector)
	if err != nil {
		log.Printf("[ERROR] [MEMORY] Failed to upsert message to Qdrant for user %s: %v", message.From.UserName, err)
		return
	}

	log.Printf("[MEMORY] Successfully processed message for user %s. Qdrant ID: %s", message.From.UserName, pointToUpsert.ID)
}


// Helper function to create a consistent hash for a user ID.
func hashUserID(userID string) string {
	hash := sha256.Sum256([]byte(userID))
	return fmt.Sprintf("%x", hash)
}