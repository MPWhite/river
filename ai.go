package main

import (
	"context"
	"encoding/json"
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

// callAnthropicForInsights makes a request to analyze notes for patterns and insights
func callAnthropicForInsights(prompt string) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	// Create Anthropic client
	client := anthropic.NewClient()

	// Construct the system prompt for insights
	systemPrompt := `You are an AI assistant specialized in analyzing personal notes to identify patterns, themes, and insights. Based on the provided notes from the last few days, provide a thoughtful analysis.

Please create the following sections:

**THEMES & PATTERNS:**
- Recurring topics or concerns that appear across multiple days
- Emotional patterns or mood trends you notice
- Productivity or energy level patterns
- Common challenges or obstacles mentioned

**KEY INSIGHTS:**
- What the notes reveal about current priorities and focus areas
- Potential blind spots or areas that might need attention
- Connections between different thoughts or ideas
- Progress or changes you can observe over time

**OBSERVATIONS:**
- Notable differences between days or shifts in thinking
- Areas where there seems to be mental clarity vs confusion
- Signs of growth, learning, or development
- Recurring questions or curiosities

Be thoughtful and nuanced in your analysis. Focus on helping the person understand their own thinking patterns and mental landscape. Cite specific examples from the notes when possible to support your observations.

Format your response with clear section headers and insightful commentary. Be encouraging and constructive while being honest about what you observe.`

	userPrompt := fmt.Sprintf("Here are my notes from the last few days:\n\n%s\n\nPlease analyze these notes for patterns, themes, and insights about my thinking and mental state.", prompt)

	// Create the message request
	ctx := context.Background()
	response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 1200,
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

// callAnthropicForSimpleTodos makes a request to extract actionable TODOs
func callAnthropicForSimpleTodos(prompt string) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	// Create Anthropic client
	client := anthropic.NewClient()

	// Construct the system prompt for simple TODOs
	systemPrompt := `You are an AI assistant that extracts actionable TODOs from personal notes. Based on the provided notes, identify clear, specific tasks that need to be completed.

Focus on:
- Explicit tasks mentioned (things to do, call, buy, schedule, etc.)
- Incomplete projects or commitments
- Follow-ups needed with people
- Deadlines or time-sensitive items
- Ideas that require action to move forward
- Problems mentioned that need solutions

For each TODO, include:
1. A clear, actionable description
2. Brief context from the notes explaining why this is needed (in parentheses)

Format as a simple numbered list. Be specific and concrete - avoid vague items. Only include things that are genuinely actionable. If no clear TODOs can be identified, say so and suggest that the person might want to be more explicit about action items in future notes.

Keep the list focused and practical - aim for quality over quantity.`

	userPrompt := fmt.Sprintf("Here are my notes from the last few days:\n\n%s\n\nPlease extract actionable TODOs from these notes.", prompt)

	// Create the message request
	ctx := context.Background()
	response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 800,
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

// callAnthropicForPrompts generates personalized journal prompts based on recent notes
func callAnthropicForPrompts(notes string) ([]string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	// Create Anthropic client
	client := anthropic.NewClient()

	// Construct the system prompt for generating personalized prompts
	systemPrompt := `You are an AI assistant that creates personalized journal prompts based on someone's recent journal entries. Your goal is to help them reflect more deeply, explore unresolved thoughts, and continue their personal growth journey.

Based on their recent notes, generate 7 thoughtful journal prompts that:

1. Build on themes and topics they've been exploring
2. Help them dig deeper into unresolved questions or concerns
3. Encourage reflection on patterns you notice
4. Challenge them to think about things from new perspectives
5. Support their goals and aspirations
6. Address any emotional or mental patterns you observe
7. Connect different ideas they've mentioned

Guidelines:
- Make prompts specific to their content, not generic
- Reference specific topics, people, or situations they've mentioned when relevant
- Vary the types of prompts (reflection, planning, gratitude, challenge, insight, etc.)
- Keep prompts open-ended but focused
- Make them thought-provoking but not overwhelming
- Consider their current emotional state and energy level

Format your response as a JSON array of strings, with exactly 7 prompts. Each prompt should be a complete question or writing prompt. Example format:
["First prompt here?", "Second prompt here?", "Third prompt here?", "Fourth prompt here?", "Fifth prompt here?", "Sixth prompt here?", "Seventh prompt here?"]`

	userPrompt := fmt.Sprintf("Here are my journal entries from the last week:\n\n%s\n\nPlease generate 7 personalized journal prompts based on these entries.", notes)

	// Create the message request
	ctx := context.Background()
	response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 800,
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
		return nil, fmt.Errorf("Anthropic API error: %v", err)
	}

	// Extract the response text
	if len(response.Content) == 0 {
		return nil, fmt.Errorf("no response content from Anthropic")
	}

	// Get the first text block from the response
	for _, content := range response.Content {
		if content.Type == "text" && content.Text != "" {
			// Parse the JSON array of prompts
			var prompts []string
			responseText := strings.TrimSpace(content.Text)

			// Find the JSON array in the response
			startIdx := strings.Index(responseText, "[")
			endIdx := strings.LastIndex(responseText, "]")

			if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
				jsonStr := responseText[startIdx : endIdx+1]

				// Try to parse as JSON array
				if err := json.Unmarshal([]byte(jsonStr), &prompts); err != nil {
					// Fallback to simple parsing if JSON fails
					jsonStr = strings.Trim(jsonStr, "[]")
					// Split by ", " but not within quotes
					parts := strings.Split(jsonStr, "\", \"")
					for _, part := range parts {
						// Clean up quotes
						part = strings.Trim(part, "\"")
						part = strings.ReplaceAll(part, "\\\"", "\"")
						if part != "" {
							prompts = append(prompts, part)
						}
					}
				}
			}

			if len(prompts) == 0 {
				return nil, fmt.Errorf("could not parse prompts from response")
			}

			return prompts, nil
		}
	}

	return nil, fmt.Errorf("unexpected response format from Anthropic")
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

// generateInsights analyzes recent notes for patterns, themes, and insights
func generateInsights() error {
	fmt.Println("🔍 Analyzing your recent notes for insights...")

	// Get recent notes (last 3 days)
	notes, err := getRecentNotes(3)
	if err != nil {
		return fmt.Errorf("error reading recent notes: %v", err)
	}

	if strings.TrimSpace(notes) == "" {
		fmt.Println("📝 No recent notes found. Try writing some thoughts first!")
		return nil
	}

	fmt.Println("🧠 Identifying patterns and themes...")

	// Call AI to generate insights
	insights, err := callAnthropicForInsights(notes)
	if err != nil {
		return fmt.Errorf("error calling AI: %v", err)
	}

	// Display the results
	fmt.Println("\n💡 Here are insights from your recent notes:")
	fmt.Println(insights)
	fmt.Println("\n💡 Tip: Add your API key with: export ANTHROPIC_API_KEY=your_key_here")

	return nil
}

// generateSimpleTodos analyzes recent notes and generates a focused list of TODOs
func generateSimpleTodos() error {
	fmt.Println("📋 Extracting TODOs from your recent notes...")

	// Get recent notes (last 3 days)
	notes, err := getRecentNotes(3)
	if err != nil {
		return fmt.Errorf("error reading recent notes: %v", err)
	}

	if strings.TrimSpace(notes) == "" {
		fmt.Println("📝 No recent notes found. Try writing some thoughts first!")
		return nil
	}

	fmt.Println("✅ Identifying actionable items...")

	// Call AI to generate simple TODOs
	todos, err := callAnthropicForSimpleTodos(notes)
	if err != nil {
		return fmt.Errorf("error calling AI: %v", err)
	}

	// Display the results
	fmt.Println("\n📝 Here are actionable TODOs from your notes:")
	fmt.Println(todos)
	fmt.Println("\n💡 Tip: Add your API key with: export ANTHROPIC_API_KEY=your_key_here")

	return nil
}

// generatePrompts analyzes recent notes and generates personalized journal prompts
func generatePrompts() error {
	fmt.Println("✨ Creating personalized prompts based on your recent writing...")

	// Get recent notes (last 7 days for more context)
	notes, err := getRecentNotes(7)
	if err != nil {
		return fmt.Errorf("error reading recent notes: %v", err)
	}

	if strings.TrimSpace(notes) == "" {
		fmt.Println("📝 No recent notes found. Try writing some thoughts first!")
		return nil
	}

	fmt.Println("🔮 Analyzing your journal entries from the last week...")

	// Call AI to generate prompts
	prompts, err := callAnthropicForPrompts(notes)
	if err != nil {
		return fmt.Errorf("error calling AI: %v", err)
	}

	// Display the prompts
	fmt.Println("\n🌟 Here are personalized journal prompts based on your recent reflections:\n")
	for i, prompt := range prompts {
		fmt.Printf("%d. %s\n\n", i+1, prompt)
	}

	// Save prompts to a file for future use
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	riverDir := filepath.Join(homeDir, "river", "notes")
	promptsFile := filepath.Join(riverDir, ".prompts")

	// Create a simple format for storing prompts with timestamps
	var promptData strings.Builder
	promptData.WriteString(fmt.Sprintf("# Generated on %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	for i, prompt := range prompts {
		promptData.WriteString(fmt.Sprintf("%d. %s\n", i+1, prompt))
	}

	if err := os.WriteFile(promptsFile, []byte(promptData.String()), 0644); err != nil {
		fmt.Printf("\n⚠️  Could not save prompts to file: %v\n", err)
	} else {
		fmt.Printf("\n💾 Prompts saved to %s\n", promptsFile)
		fmt.Println("   These prompts will be used for your daily notes over the next week.")
	}

	fmt.Println("\n💡 Tip: Run 'river prompts' weekly to get fresh, personalized prompts!")

	return nil
}
