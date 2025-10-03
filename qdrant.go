package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var qdrantPointsClient qdrant.PointsClient

func initQdrant() error {
	qdrantUrl := os.Getenv("QDRANT_URL")
	if qdrantUrl == "" {
		qdrantUrl = "http://qdrant:6333"
	}

	parsedUrl, err := url.Parse(qdrantUrl)
	if err != nil {
		return fmt.Errorf("failed to parse QDRANT_URL: %w", err)
	}

	host := parsedUrl.Host
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
		perRpcCreds = grpc.WithPerRPCCredentials(perRPCCredentials{
			apiKey: apiKey,
			useTls: useTls,
		})
	}

	var conn *grpc.ClientConn
	if perRpcCreds != nil {
		conn, err = grpc.Dial(host, creds, perRpcCreds)
	} else {
		conn, err = grpc.Dial(host, creds)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to Qdrant: %w", err)
	}

	qdrantPointsClient = qdrant.NewPointsClient(conn)
	log.Printf("Qdrant client initialized with URL: %s", qdrantUrl)
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

type perRPCCredentials struct {
	apiKey string
	useTls bool
}

func (p perRPCCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"api-key": p.apiKey,
	}, nil
}

func (p perRPCCredentials) RequireTransportSecurity() bool {
	return p.useTls
}