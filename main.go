package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	OutputDir   string  `json:"OutputDir"`
	RateLimit   float64 `json:"rateLimit"`
	MaxRetries  int     `json:"MaxRetries"`
	MaxParallel int     `json:"MaxParallel"`
	UrlFile     string  `json:"UrlFile"`
}

func main() {
	file, _ := os.ReadFile("config.json")
	var config Config
	json.Unmarshal(file, &config)

	fmt.Println(config)

	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		fmt.Println("Error creating output directory:", err)
		return
	}

}
