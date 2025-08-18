package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mattwhite/river-go/internal/ai"
	"github.com/mattwhite/river-go/internal/editor"
	"github.com/mattwhite/river-go/internal/onboarding"
	"github.com/mattwhite/river-go/internal/statsui"
)

func printHelp() {
	fmt.Println("🌊 River - A minimalist daily journaling application")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  river              Start the journal editor")
	fmt.Println("  river stats        View writing statistics dashboard")
	fmt.Println("  river onboard      Set up AI features (API key)")
	fmt.Println()
	fmt.Println("AI Commands (requires API key):")
	fmt.Println("  river prompts      Generate personalized journal prompts")
	fmt.Println("  river think        Generate categorized TODOs from recent notes")
	fmt.Println("  river analyze      Get insights and patterns from recent notes")
	fmt.Println("  river todo         Extract simple actionable items from notes")
	fmt.Println()
	fmt.Println("Other:")
	fmt.Println("  river help         Show this help message")
	fmt.Println()
	fmt.Println("First time? Run 'river onboard' to set up AI features.")
}

func main() {
	// Check if this is the first run and API key is needed
	if onboarding.NeedsOnboarding() && len(os.Args) == 1 {
		fmt.Println("🌊 Welcome to River!")
		fmt.Println("\nIt looks like this is your first time running River.")
		fmt.Println("Would you like to set up AI features? (You can do this later with 'river onboard')")
		fmt.Print("\nPress Enter to continue or Ctrl+C to skip: ")
		fmt.Scanln()
		if err := onboarding.RunOnboarding(); err != nil {
			fmt.Printf("Setup error: %v\n", err)
		}
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "stats":
			p := tea.NewProgram(statsui.InitModel(), tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				fmt.Printf("Error: %v", err)
				os.Exit(1)
			}
			return
		case "think":
			if err := ai.GenerateTodos(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "analyze":
			if err := ai.GenerateInsights(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "todo":
			if err := ai.GenerateSimpleTodos(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "prompts":
			if err := ai.GeneratePrompts(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "onboard":
			if err := onboarding.RunOnboarding(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "help", "--help", "-h":
			printHelp()
			return
		default:
			fmt.Printf("Unknown command: %s\n\n", os.Args[1])
			printHelp()
			os.Exit(1)
		}
	}

	// Default behavior - run the note editor
	p := tea.NewProgram(editor.NewInitialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
