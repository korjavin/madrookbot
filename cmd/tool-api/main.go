package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/qdrant/go-client/qdrant"
	openai "github.com/sashabaranov/go-openai"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	openaiClient       *openai.Client
	qdrantPointsClient qdrant.PointsClient
)

type SearchRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}

type SearchResult struct {
	Text         string  `json:"text"`
	TelegramLink string  `json:"telegram_link"`
	TimestampUTC string  `json:"timestamp_utc"`
	Score        float32 `json:"score"`
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
}

func main() {
	// Initialize OpenAI client for embeddings
	if os.Getenv("GPT_TOKEN") != "" {
		openaiClient = openai.NewClient(os.Getenv("GPT_TOKEN"))
		log.Printf("[INFO] OpenAI client initialized")
	} else {
		log.Fatal("GPT_TOKEN environment variable not set")
	}

	// Initialize Qdrant client
	if err := initQdrant(); err != nil {
		log.Fatalf("Error initializing Qdrant: %v", err)
	}

	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/health", healthHandler)

	port := os.Getenv("TOOL_API_PORT")
	if port == "" {
		port = "8081"
	}
	log.Printf("[INFO] Tool API server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(w, "OK"); err != nil {
		log.Printf("[ERROR] Failed to write health response: %v", err)
	}
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "Query cannot be empty", http.StatusBadRequest)
		return
	}

	if req.TopK == 0 {
		req.TopK = 10
	}

	log.Printf("[INFO] Received search query: '%s', top_k: %d", req.Query, req.TopK)

	// Generate embedding for query
	embedding, err := getEmbedding(req.Query)
	if err != nil {
		log.Printf("[ERROR] Failed to get embedding: %v", err)
		http.Error(w, "Failed to get embedding", http.StatusInternalServerError)
		return
	}

	// Search in Qdrant
	searchResults, err := searchQdrant(embedding, req.TopK)
	if err != nil {
		log.Printf("[ERROR] Failed to search Qdrant: %v", err)
		http.Error(w, "Failed to search Qdrant", http.StatusInternalServerError)
		return
	}

	// Format and return results
	response := formatQdrantResults(searchResults)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[ERROR] Failed to encode response: %v", err)
	}
}

func getEmbedding(text string) ([]float32, error) {
	ctx := context.Background()
	embeddingModel := os.Getenv("OPENAI_EMBEDDING_MODEL")
	if embeddingModel == "" {
		embeddingModel = "text-embedding-ada-002"
	}

	req := openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.EmbeddingModel(embeddingModel),
	}

	resp, err := openaiClient.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Data) > 0 {
		return resp.Data[0].Embedding, nil
	}

	return nil, fmt.Errorf("no embedding returned")
}

func initQdrant() error {
	qdrantUrl := os.Getenv("QDRANT_URL")
	if qdrantUrl == "" {
		qdrantUrl = "http://qdrant:6333"
	}

	parsedUrl, err := url.Parse(qdrantUrl)
	if err != nil {
		return fmt.Errorf("failed to parse QDRANT_URL: %w", err)
	}

	// For gRPC, use port 6334 (default Qdrant gRPC port)
	host := parsedUrl.Hostname()
	port := "6334"
	if parsedUrl.Port() != "" && parsedUrl.Port() != "6333" {
		port = parsedUrl.Port()
	}
	grpcAddr := fmt.Sprintf("%s:%s", host, port)

	useTls := parsedUrl.Scheme == "https"

	var creds grpc.DialOption
	if useTls {
		creds = grpc.WithTransportCredentials(credentials.NewTLS(nil))
	} else {
		creds = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	apiKey := os.Getenv("QDRANT_API_KEY")
	var perRpcCreds grpc.DialOption
	if apiKey != "" {
		perRpcCreds = grpc.WithPerRPCCredentials(perRPCCredentials{apiKey: apiKey, useTLS: useTls})
	}

	var conn *grpc.ClientConn
	if perRpcCreds != nil {
		conn, err = grpc.Dial(grpcAddr, creds, perRpcCreds)
	} else {
		conn, err = grpc.Dial(grpcAddr, creds)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to Qdrant at %s: %w", grpcAddr, err)
	}

	qdrantPointsClient = qdrant.NewPointsClient(conn)
	log.Printf("[INFO] Qdrant client initialized with gRPC address: %s", grpcAddr)
	return nil
}

func searchQdrant(embedding []float32, topK int) ([]*qdrant.ScoredPoint, error) {
	collectionName := os.Getenv("QDRANT_COLLECTION_NAME")
	if collectionName == "" {
		collectionName = "telegram_messages"
	}

	searchRequest := &qdrant.SearchPoints{
		CollectionName: collectionName,
		Vector:         embedding,
		Limit:          uint64(topK),
		WithPayload: &qdrant.WithPayloadSelector{
			SelectorOptions: &qdrant.WithPayloadSelector_Include{
				Include: &qdrant.PayloadIncludeSelector{
					Fields: []string{"text", "telegram_link", "timestamp_utc"},
				},
			},
		},
	}

	res, err := qdrantPointsClient.Search(context.Background(), searchRequest)
	if err != nil {
		return nil, err
	}

	return res.GetResult(), nil
}

func formatQdrantResults(points []*qdrant.ScoredPoint) SearchResponse {
	var results []SearchResult
	for _, point := range points {
		payload := point.GetPayload()
		if payload == nil {
			continue
		}

		var textVal, telegramLinkVal, timestampUtcVal string
		if text, ok := payload["text"]; ok {
			textVal = text.GetStringValue()
		}
		if telegramLink, ok := payload["telegram_link"]; ok {
			telegramLinkVal = telegramLink.GetStringValue()
		}
		if timestampUtc, ok := payload["timestamp_utc"]; ok {
			timestampUtcVal = timestampUtc.GetStringValue()
		}

		results = append(results, SearchResult{
			Text:         textVal,
			TelegramLink: telegramLinkVal,
			TimestampUTC: timestampUtcVal,
			Score:        point.GetScore(),
		})
	}
	return SearchResponse{Results: results}
}

type perRPCCredentials struct {
	apiKey string
	useTLS bool
}

func (p perRPCCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"api-key": p.apiKey,
	}, nil
}

func (p perRPCCredentials) RequireTransportSecurity() bool {
	return p.useTLS
}
