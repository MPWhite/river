package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OpenAI API structures
type OpenAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	MaxTokens int      `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIResponse struct {
	Choices []Choice `json:"choices"`
	Error   *APIError `json:"error,omitempty"`
}

type Choice struct {
	Message Message `json:"message"`
}

type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// getRecentNotes reads notes from the last few days
func getRecentNotes(days int) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	riverDir := filepath.Join(homeDir, "river", "notes")
	
	var allContent strings.Builder
	
	// Get the last 'days' worth of notes
	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		filename := filepath.Join(riverDir, dateStr+".md")
		
		content, err := os.ReadFile(filename)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Skip missing files
			}
			return "", err
		}
		
		// Filter out HTML comments (ghost text)
		lines := strings.Split(string(content), "\n")
		var filteredLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->") {
				continue // Skip ghost text
			}
			if strings.TrimSpace(line) != "" { // Only include non-empty lines
				filteredLines = append(filteredLines, line)
			}
		}
		
		if len(filteredLines) > 0 {
			allContent.WriteString(fmt.Sprintf("\n=== %s ===\n", date.Format("Monday, January 2, 2006")))
			allContent.WriteString(strings.Join(filteredLines, "\n"))
			allContent.WriteString("\n")
		}
	}
	
	return allContent.String(), nil
}

// callOpenAI makes a request to the OpenAI API
func callOpenAI(prompt string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}
	
	// Construct the full prompt
	systemPrompt := `You are an AI assistant helping with personal productivity. Based on the provided notes from the last few days, generate a list of actionable TODOs. 

Focus on:
- Incomplete tasks or projects mentioned
- Follow-ups needed 
- Ideas that could be developed further
- Goals or commitments that need action
- Any deadlines or time-sensitive items

Format your response as a simple numbered list of actionable items. Be specific and concise. If no clear TODOs can be derived, suggest a few general productivity actions based on the content themes.`

	userPrompt := fmt.Sprintf("Here are my notes from the last few days:\n\n%s\n\nPlease generate a list of TODOs based on this content.", prompt)
	
	request := OpenAIRequest{
		Model: "gpt-3.5-turbo",
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 500,
		Temperature: 0.7,
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	var openAIResp OpenAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return "", err
	}
	
	if openAIResp.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", openAIResp.Error.Message)
	}
	
	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}
	
	return openAIResp.Choices[0].Message.Content, nil
}

// generateTodos analyzes recent notes and generates TODOs using AI
func generateTodos() error {
	fmt.Println("🤔 Thinking about your recent notes...")
	
	// Get recent notes (last 3 days)
	notes, err := getRecentNotes(3)
	if err != nil {
		return fmt.Errorf("error reading recent notes: %v", err)
	}
	
	if strings.TrimSpace(notes) == "" {
		fmt.Println("📝 No recent notes found. Try writing some thoughts first!")
		return nil
	}
	
	fmt.Println("📖 Analyzing notes from the last 3 days...")
	
	// Call AI to generate TODOs
	todos, err := callOpenAI(notes)
	if err != nil {
		return fmt.Errorf("error calling AI: %v", err)
	}
	
	// Display the results
	fmt.Println("\n✨ Here are some TODOs based on your recent notes:")
	fmt.Println(todos)
	fmt.Println("\n💡 Tip: Add your API key with: export OPENAI_API_KEY=your_key_here")
	
	return nil
}