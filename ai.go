package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

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

// callAnthropic makes a request to the Anthropic API using Claude
func callAnthropic(prompt string) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}
	
	// Create Anthropic client
	client := anthropic.NewClient()
	
	// Construct the system prompt
	systemPrompt := `You are an AI assistant helping with personal productivity. Based on the provided notes from the last few days, generate organized lists of TODOs in different categories.

Create the following sections:

**WORK TODOs:**
- Tasks related to professional projects, meetings, deadlines
- Follow-ups with colleagues or clients
- Work-related goals and commitments

**PERSONAL TODOs:**
- Personal tasks, household items, family commitments
- Health, fitness, and self-care items
- Financial and administrative tasks

**PROJECTS & IDEAS:**
- Ideas that need further exploration or research
- Long-term goals and strategic thinking
- Creative projects or learning opportunities
- Reflection and planning items

Focus on:
- Incomplete tasks or projects mentioned
- Follow-ups needed 
- Ideas that could be developed further
- Goals or commitments that need action
- Any deadlines or time-sensitive items

IMPORTANT: For each TODO item, include a brief rationale in parentheses that cites or references the specific note content that led to this suggestion. For example:
"1. Follow up with John about the project proposal (mentioned meeting him on Tuesday but no follow-up scheduled)"

Format your response with clear section headers and numbered lists under each. Be specific and concise. If a category has no clear TODOs, you may omit that section or suggest general productivity actions based on the content themes.`

	userPrompt := fmt.Sprintf("Here are my notes from the last few days:\n\n%s\n\nPlease generate organized lists of TODOs based on this content, categorized by Work, Home, and Deeper Thought items.", prompt)
	
	// Create the message request
	ctx := context.Background()
	response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     "claude-sonnet-4-20250514", // Using Haiku for faster/cheaper responses
		MaxTokens: 1000,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt, Type: "text"}},
		Messages: []anthropic.MessageParam{
			{
				Role: anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{
					{
						OfText: &anthropic.TextBlockParam{
							Text: userPrompt,
							Type: "text",
						},
					},
				},
			},
		},
	})
	
	if err != nil {
		return "", fmt.Errorf("Anthropic API error: %v", err)
	}
	
	// Extract the response text
	if len(response.Content) == 0 {
		return "", fmt.Errorf("no response content from Anthropic")
	}
	
	// Get the first text block from the response
	for _, content := range response.Content {
		if content.Type == "text" && content.Text != "" {
			return content.Text, nil
		}
	}
	
	return "", fmt.Errorf("unexpected response format from Anthropic")
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
	todos, err := callAnthropic(notes)
	if err != nil {
		return fmt.Errorf("error calling AI: %v", err)
	}
	
	// Display the results
	fmt.Println("\n✨ Here are some TODOs based on your recent notes:")
	fmt.Println(todos)
	fmt.Println("\n💡 Tip: Add your API key with: export ANTHROPIC_API_KEY=your_key_here")
	
	return nil
}