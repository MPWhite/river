# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

River is a minimalist daily journaling application written in Go using the Bubble Tea framework. It provides a distraction-free writing environment with automatic daily note management, word count tracking, typing time measurement, and AI-powered analysis features.

## Key Commands

### Build & Run
- `go build -o river` - Build the main binary
- `./river` - Run the journal editor
- `./river stats` - View writing statistics dashboard
- `./river think` - Generate categorized TODOs using AI
- `./river analyze` - Get AI insights from recent notes
- `./river todo` - Extract simple actionable items

### Development
- `go fmt ./...` - Format code
- `go mod tidy` - Clean up dependencies
- `golangci-lint run` - Run linter (if installed)

## Architecture & Components

### Main Components
1. **main.go** - Core editor functionality with Bubble Tea TUI
   - Daily note management (auto-creates in ~/river/notes/)
   - Real-time word counting and typing time tracking
   - Progress bar visualization
   - Auto-save functionality

2. **stats.go** - Statistics dashboard
   - Multi-tab interface (Overview, Daily, Weekly, Trends)
   - Streak tracking and productivity metrics
   - Visual progress bars and charts

3. **ai.go** - AI integration features
   - Uses Anthropic Claude API (requires ANTHROPIC_API_KEY env var)
   - Three analysis modes: categorized TODOs, insights, simple TODOs
   - Processes last 3 days of notes

### Data Storage
- Notes stored in: `~/river/notes/YYYY-MM-DD.md`
- Stats stored in: `~/river/notes/.stats-YYYY-MM-DD.toml`
- Daily note template includes date and rotating prompt

### Key Implementation Details
- Uses Bubble Tea for terminal UI with Model-Update-View pattern
- Implements custom cursor rendering and viewport scrolling
- Ghost text (prompts) rendered as HTML comments
- Real-time typing activity tracking (1-minute timeout)
- Word counting excludes ghost text
- Progress tracking against 500-word daily target