//go:build toolapi

package main

import (
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"github.com/stretchr/testify/assert"
)

func TestFormatQdrantResults(t *testing.T) {
	points := []*qdrant.ScoredPoint{
		{
			Id: &qdrant.PointId{PointIdOptions: &qdrant.PointId_Num{Num: 1}},
			Payload: map[string]*qdrant.Value{
				"text":          {Kind: &qdrant.Value_StringValue{StringValue: "test text 1"}},
				"telegram_link": {Kind: &qdrant.Value_StringValue{StringValue: "link1"}},
				"timestamp_utc": {Kind: &qdrant.Value_StringValue{StringValue: "2024-01-01"}},
			},
			Score: 0.9,
		},
		{
			Id: &qdrant.PointId{PointIdOptions: &qdrant.PointId_Num{Num: 2}},
			Payload: map[string]*qdrant.Value{
				"text":          {Kind: &qdrant.Value_StringValue{StringValue: "test text 2"}},
				"telegram_link": {Kind: &qdrant.Value_StringValue{StringValue: "link2"}},
				"timestamp_utc": {Kind: &qdrant.Value_StringValue{StringValue: "2024-01-02"}},
			},
			Score: 0.8,
		},
	}

	expected := SearchResponse{
		Results: []SearchResult{
			{Text: "test text 1", TelegramLink: "link1", TimestampUTC: "2024-01-01", Score: 0.9},
			{Text: "test text 2", TelegramLink: "link2", TimestampUTC: "2024-01-02", Score: 0.8},
		},
	}

	actual := formatQdrantResults(points)
	assert.Equal(t, expected, actual)
}