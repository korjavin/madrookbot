//go:build toolapi

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/qdrant/go-client/qdrant"
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
	// Initialize OpenAI client
	if os.Getenv("GPT_TOKEN") != "" {
		initGPT()
	} else {
		log.Fatal("GPT_TOKEN environment variable not set")
	}

	// Initialize Qdrant client
	if err := initQdrant(); err != nil {
		log.Fatalf("Error initializing Qdrant: %v", err)
	}

	http.HandleFunc("/search", searchHandler)
	port := os.Getenv("TOOL_API_PORT")
	if port == "" {
		port = "8081"
	}
	log.Printf("Tool API server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
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
		req.TopK = 3
	}

	log.Printf("Received search query: '%s', top_k: %d", req.Query, req.TopK)

	// 1. Generate embedding for req.Query
	embedding, err := getEmbedding(req.Query)
	if err != nil {
		log.Printf("Failed to get embedding: %v", err)
		http.Error(w, "Failed to get embedding", http.StatusInternalServerError)
		return
	}

	// 2. Search in Qdrant
	searchResults, err := searchQdrant(embedding, req.TopK)
	if err != nil {
		log.Printf("Failed to search Qdrant: %v", err)
		http.Error(w, "Failed to search Qdrant", http.StatusInternalServerError)
		return
	}

	// 3. Format and return results
	response := formatQdrantResults(searchResults)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
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