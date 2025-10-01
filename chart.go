package main

import (
	"bytes"
	"fmt"
	"image/color"
	"sort"
	"time"

	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

// generateActivityChart creates a PNG chart showing user activity over time
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

	// Prepare data for stacked area chart
	var series []chart.Series

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

	// Create time series for each user
	for idx, userStat := range stats.UserStats {
		// Get all unique hour buckets for x-axis
		var xValues []time.Time
		var yValues []float64

		// Create sorted time buckets
		var allHours []time.Time
		for hour := range userStat.HourlyData {
			allHours = append(allHours, hour)
		}
		sort.Slice(allHours, func(i, j int) bool {
			return allHours[i].Before(allHours[j])
		})

		for _, hour := range allHours {
			xValues = append(xValues, hour)
			yValues = append(yValues, float64(userStat.HourlyData[hour]))
		}

		userColor := colors[idx%len(colors)]
		fillColor := color.RGBA{
			R: userColor.R,
			G: userColor.G,
			B: userColor.B,
			A: 180,
		}
		series = append(series, chart.TimeSeries{
			Name: userStat.UserName,
			Style: chart.Style{
				StrokeColor: drawing.Color(userColor),
				FillColor:   drawing.Color(fillColor),
				StrokeWidth: 2,
			},
			XValues: xValues,
			YValues: yValues,
		})
	}

	graph := chart.Chart{
		Title: fmt.Sprintf("Group Activity (Last 7 Days)"),
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
		XAxis: chart.XAxis{
			Name: "Time",
			Style: chart.Style{
				FontSize: 10,
			},
			ValueFormatter: chart.TimeHourValueFormatter,
		},
		YAxis: chart.YAxis{
			Name: "Messages",
			Style: chart.Style{
				FontSize: 10,
			},
		},
		Series: series,
	}

	// Add legend
	graph.Elements = []chart.Renderable{
		chart.LegendThin(&graph),
	}

	// Render to buffer
	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
	if err != nil {
		return nil, err
	}

	return buffer, nil
}
