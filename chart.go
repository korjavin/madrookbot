package main

import (
	"bytes"
	"fmt"
	"image/color"
	"sort"

	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

// generateActivityChart creates a PNG bar chart showing user message counts
func generateActivityChart(stats *ActivityStats) (*bytes.Buffer, error) {
	if len(stats.UserStats) == 0 {
		return nil, fmt.Errorf("no activity data to chart")
	}

	// Sort users by total messages (top contributors first)
	sort.Slice(stats.UserStats, func(i, j int) bool {
		return stats.UserStats[i].TotalMessages > stats.UserStats[j].TotalMessages
	})

	// Limit to top 10 users for readability
	maxUsers := 10
	if len(stats.UserStats) > maxUsers {
		stats.UserStats = stats.UserStats[:maxUsers]
	}

	// Prepare data for bar chart
	var bars []chart.Value

	// Color palette
	colors := []color.RGBA{
		{85, 168, 104, 255},  // green
		{78, 121, 167, 255},  // blue
		{242, 142, 43, 255},  // orange
		{225, 87, 89, 255},   // red
		{118, 113, 113, 255}, // gray
		{237, 201, 72, 255},  // yellow
		{176, 122, 161, 255}, // purple
		{156, 117, 95, 255},  // brown
		{186, 176, 172, 255}, // tan
		{255, 157, 167, 255}, // pink
	}

	// Create bars for each user
	for idx, userStat := range stats.UserStats {
		userColor := colors[idx%len(colors)]
		bars = append(bars, chart.Value{
			Label: userStat.UserName,
			Value: float64(userStat.TotalMessages),
			Style: chart.Style{
				StrokeColor: drawing.Color(userColor),
				FillColor:   drawing.Color(userColor),
				StrokeWidth: 0,
			},
		})
	}

	graph := chart.BarChart{
		Title: "Group Activity (Last 7 Days)",
		TitleStyle: chart.Style{
			FontSize: 16,
		},
		Width:  1200,
		Height: 600,
		Background: chart.Style{
			Padding: chart.Box{
				Top:    50,
				Left:   20,
				Right:  20,
				Bottom: 20,
			},
		},
		XAxis: chart.Style{
			FontSize: 10,
		},
		YAxis: chart.YAxis{
			Name: "Number of Messages",
			Style: chart.Style{
				FontSize: 10,
			},
		},
		BarWidth: 80,
		Bars:     bars,
	}

	// Render to buffer
	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
	if err != nil {
		return nil, err
	}

	return buffer, nil
}
