package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// WeatherTool provides a simple fake weather service
type WeatherTool struct{}

// WeatherResult represents the result of a weather query
type WeatherResult struct {
	City        string `json:"city"`
	Temperature int    `json:"temperature"`
	Condition   string `json:"condition"`
	Humidity    int    `json:"humidity"`
	WindSpeed   int    `json:"wind_speed"`
	Description string `json:"description"`
}

// GetWeather returns fake weather data for a given city
// This function will be registered with the toolbox and called by the AI
func (w *WeatherTool) GetWeather(city string) (*WeatherResult, error) {
	if city == "" {
		return nil, fmt.Errorf("city name is required")
	}

	// Normalize city name
	city = strings.TrimSpace(city)
	cityLower := strings.ToLower(city)

	// Use city name to seed random generator for consistent results
	seed := int64(0)
	for _, char := range cityLower {
		seed += int64(char)
	}
	rng := rand.New(rand.NewSource(seed))

	// Generate fake weather data based on city "characteristics"
	conditions := []string{"sunny", "cloudy", "partly cloudy", "overcast", "light rain", "heavy rain", "snow", "fog"}
	condition := conditions[rng.Intn(len(conditions))]

	// Temperature range based on "geographic" characteristics of city name
	var tempBase int
	switch {
	case strings.Contains(cityLower, "miami") || strings.Contains(cityLower, "phoenix") || strings.Contains(cityLower, "las vegas"):
		tempBase = 85
	case strings.Contains(cityLower, "seattle") || strings.Contains(cityLower, "portland") || strings.Contains(cityLower, "vancouver"):
		tempBase = 65
	case strings.Contains(cityLower, "chicago") || strings.Contains(cityLower, "boston") || strings.Contains(cityLower, "new york"):
		tempBase = 70
	case strings.Contains(cityLower, "san francisco"):
		tempBase = 68
	case strings.Contains(cityLower, "london") || strings.Contains(cityLower, "paris"):
		tempBase = 62
	case strings.Contains(cityLower, "moscow") || strings.Contains(cityLower, "alaska"):
		tempBase = 45
	default:
		tempBase = 72 // Default moderate temperature
	}

	temperature := tempBase + rng.Intn(21) - 10 // ±10 degrees variation
	humidity := 30 + rng.Intn(50)               // 30-80% humidity
	windSpeed := rng.Intn(25)                   // 0-25 mph wind

	// Generate description based on condition
	var description string
	switch condition {
	case "sunny":
		description = "Clear skies with plenty of sunshine"
	case "cloudy":
		description = "Overcast with thick cloud cover"
	case "partly cloudy":
		description = "Mix of sun and clouds throughout the day"
	case "overcast":
		description = "Gray skies with complete cloud cover"
	case "light rain":
		description = "Gentle rainfall, perfect for staying indoors"
	case "heavy rain":
		description = "Intense rainfall, bring an umbrella!"
	case "snow":
		description = "Snow falling, winter wonderland conditions"
	case "fog":
		description = "Dense fog reducing visibility"
	default:
		description = "Weather conditions as expected"
	}

	result := &WeatherResult{
		City:        city,
		Temperature: temperature,
		Condition:   condition,
		Humidity:    humidity,
		WindSpeed:   windSpeed,
		Description: description,
	}

	// Simulate some processing time
	time.Sleep(100 * time.Millisecond)

	return result, nil
}
