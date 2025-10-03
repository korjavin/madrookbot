package importer

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/qdrant/go-client/qdrant"
)

const (
	// OpenAI's text-embedding-ada-002 model output dimension
	vectorDimension = 1536
	// Qdrant collection name
	collectionName = "telegram_messages"
)

// TelegramExport represents the top-level structure of a Telegram JSON export.
type TelegramExport struct {
	ChatID   int       `json:"id"`
	Name     string    `json:"name"`
	Messages []Message `json:"messages"`
}

// Message represents a single message within the export.
type Message struct {
	ID           int          `json:"id"`
	Type         string       `json:"type"`
	Date         string       `json:"date"`
	DateUnixtime string       `json:"date_unixtime"`
	FromID       string       `json:"from_id"`
	Text         interface{}  `json:"text"`
	TextEntities []TextEntity `json:"text_entities"`
}

// TextEntity represents a part of a message's text, which can be plain or formatted.
type TextEntity struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// GetText concatenates text from TextEntities or returns the simple text field.
func (m *Message) GetText() string {
	if len(m.TextEntities) > 0 {
		var builder strings.Builder
		for _, entity := range m.TextEntities {
			builder.WriteString(entity.Text)
		}
		return builder.String()
	}
	if text, ok := m.Text.(string); ok {
		return text
	}
	if textSlice, ok := m.Text.([]interface{}); ok {
		var builder strings.Builder
		for _, item := range textSlice {
			if str, ok := item.(string); ok {
				builder.WriteString(str)
			} else if obj, ok := item.(map[string]interface{}); ok {
				if txt, ok := obj["text"].(string); ok {
					builder.WriteString(txt)
				}
			}
		}
		return builder.String()
	}
	return ""
}

func Run() {
	inputFile := flag.String("input", "", "Path to Telegram JSON export file")
	batchSize := flag.Int("batch-size", 50, "Number of messages to process in a batch")
	minMessageLength := flag.Int("min-length", 15, "Minimum character length for a message to be imported")
	maxMessages := flag.Int("max-messages", 0, "Maximum number of messages to import (0 = no limit)")
	flag.Parse()

	if *inputFile == "" {
		log.Fatal("Input file path is required. Use the -input flag.")
	}

	log.Printf("Starting import from file: %s", *inputFile)
	log.Printf("Batch size: %d, Min message length: %d", *batchSize, *minMessageLength)

	file, err := os.Open(*inputFile)
	if err != nil {
		log.Fatalf("Failed to open input file: %v", err)
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	var export TelegramExport
	if err := json.Unmarshal(byteValue, &export); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	log.Printf("Successfully parsed %d messages from chat '%s' (ID: %d)", len(export.Messages), export.Name, export.ChatID)

	// Apply max messages limit if specified
	if *maxMessages > 0 && len(export.Messages) > *maxMessages {
		log.Printf("Limiting to first %d messages (out of %d total)", *maxMessages, len(export.Messages))
		export.Messages = export.Messages[:*maxMessages]
	}

	// Initialize OpenAI client
	openaiToken := os.Getenv("GPT_TOKEN")
	if openaiToken == "" {
		log.Fatal("GPT_TOKEN environment variable not set.")
	}
	openaiClient := openai.NewClient(openaiToken)

	// Initialize Qdrant client
	qdrantAddr := os.Getenv("QDRANT_HOST")
	if qdrantAddr == "" {
		log.Fatal("QDRANT_HOST environment variable not set.")
	}
	qdrantHost := qdrantAddr
	qdrantPort := 6334 // default qdrant grpc port
	if strings.Contains(qdrantAddr, ":") {
		parts := strings.Split(qdrantAddr, ":")
		qdrantHost = parts[0]
		qdrantPort, err = strconv.Atoi(parts[1])
		if err != nil {
			log.Fatalf("Invalid port in QDRANT_HOST: %v", err)
		}
	}

	qdrantApiKey := os.Getenv("QDRANT_API_KEY")

	qdrantClient, err := qdrant.NewClient(&qdrant.Config{
		Host:   qdrantHost,
		Port:   qdrantPort,
		APIKey: qdrantApiKey,
	})
	if err != nil {
		log.Fatalf("Failed to create Qdrant client: %v", err)
	}
	defer qdrantClient.Close()

	// Ensure collection exists
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	collectionExists, err := qdrantClient.CollectionExists(ctx, collectionName)
	if err != nil {
		log.Fatalf("Failed to check if collection exists: %v", err)
	}

	if !collectionExists {
		log.Printf("Collection '%s' does not exist. Creating it...", collectionName)
		err := qdrantClient.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: collectionName,
			VectorsConfig: &qdrant.VectorsConfig{
				Config: &qdrant.VectorsConfig_Params{
					Params: &qdrant.VectorParams{
						Size:     vectorDimension,
						Distance: qdrant.Distance_Cosine,
					},
				},
			},
		})
		if err != nil {
			log.Fatalf("Failed to create collection: %v", err)
		}
		log.Printf("Collection '%s' created successfully.", collectionName)
	} else {
		log.Printf("Using existing collection: '%s'", collectionName)
	}

	channelID := strings.TrimPrefix(strconv.Itoa(export.ChatID), "-100")

	// Process messages in batches
	pointsClient := qdrantClient.GetPointsClient()
	for i := 0; i < len(export.Messages); i += *batchSize {
		end := i + *batchSize
		if end > len(export.Messages) {
			end = len(export.Messages)
		}
		batch := export.Messages[i:end]

		log.Printf("Processing batch %d-%d...", i, end-1)

		var textsToEmbed []string
		var messagesToKeep []Message

		for _, msg := range batch {
			if msg.Type != "message" {
				continue
			}
			text := msg.GetText()
			if len(text) < *minMessageLength {
				continue
			}
			textsToEmbed = append(textsToEmbed, text)
			messagesToKeep = append(messagesToKeep, msg)
		}

		if len(textsToEmbed) == 0 {
			log.Println("No valid messages in this batch, skipping.")
			continue
		}

		log.Printf("Getting embeddings for %d messages...", len(textsToEmbed))

		embeddingCtx, embeddingCancel := context.WithTimeout(context.Background(), time.Minute)

		resp, err := openaiClient.CreateEmbeddings(
			embeddingCtx,
			openai.EmbeddingRequest{
				Input: textsToEmbed,
				Model: openai.AdaEmbeddingV2,
			},
		)
		embeddingCancel()
		if err != nil {
			log.Printf("Error getting embeddings for batch %d-%d: %v", i, end-1, err)
			continue
		}

		log.Printf("Got %d embeddings. Upserting to Qdrant...", len(resp.Data))

		points := make([]*qdrant.PointStruct, 0, len(messagesToKeep))
		for j, msg := range messagesToKeep {
			timestamp, _ := time.Parse(time.RFC3339, msg.Date+"Z")

			payload := map[string]*qdrant.Value{
				"message_id":    {Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(msg.ID)}},
				"user_id":       {Kind: &qdrant.Value_StringValue{StringValue: msg.FromID}},
				"text":          {Kind: &qdrant.Value_StringValue{StringValue: textsToEmbed[j]}},
				"timestamp_utc": {Kind: &qdrant.Value_StringValue{StringValue: timestamp.UTC().Format(time.RFC3339)}},
				"telegram_link": {Kind: &qdrant.Value_StringValue{StringValue: fmt.Sprintf("https://t.me/c/%s/%d", channelID, msg.ID)}},
			}

			points = append(points, &qdrant.PointStruct{
				Id: &qdrant.PointId{
					PointIdOptions: &qdrant.PointId_Num{Num: uint64(msg.ID)},
				},
				Payload: payload,
				Vectors: &qdrant.Vectors{
					VectorsOptions: &qdrant.Vectors_Vector{
						Vector: &qdrant.Vector{
							Data: resp.Data[j].Embedding,
						},
					},
				},
			})
		}

		upsertCtx, upsertCancel := context.WithTimeout(context.Background(), 2*time.Minute)

		wait := true
		_, err = pointsClient.Upsert(upsertCtx, &qdrant.UpsertPoints{
			CollectionName: collectionName,
			Wait:           &wait,
			Points:         points,
		})
		upsertCancel()

		if err != nil {
			log.Printf("Error upserting points for batch %d-%d: %v", i, end-1, err)
			continue
		}

		log.Printf("Successfully upserted %d points to Qdrant.", len(points))
	}

	log.Println("Importer finished processing all messages.")
}
