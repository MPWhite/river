# Publishing River as an NPM Package

This guide explains how to publish your Go binary as an npm package for easy installation via `npm install -g river-writer`.

## Setup Steps

### 1. Update Package Configuration
Edit `package.json`:
- The package is named `river-writer`
- Update `repository.url` with your GitHub repository
- Add your name to the `author` field
- Adjust version as needed

Edit `scripts/install.js`:
- Update `REPO_OWNER` with your GitHub username
- Update `REPO_NAME` if different

Edit `.goreleaser.yml`:
- Update the GitHub owner and repository name

### 2. Create GitHub Release with Binaries

Install GoReleaser:
```bash
brew install goreleaser
# or
go install github.com/goreleaser/goreleaser@latest
```

Create and push a git tag:
```bash
git tag v1.0.0
git push origin v1.0.0
```

Build and release:
```bash
goreleaser release --clean
```

This creates a GitHub release with pre-built binaries for all platforms.

### 3. Test Locally

Test the npm package locally:
```bash
npm pack
npm install -g river-writer-0.0.1.tgz
river
```

### 4. Publish to NPM

Create an npm account at https://www.npmjs.com if you don't have one.

Login to npm:
```bash
npm login
```

Publish the package:
```bash
npm publish
```

## How It Works

1. **Installation Flow**:
   - User runs `npm install -g river-writer`
   - npm installs the package globally
   - `postinstall` script runs automatically
   - Script attempts to download pre-built binary from GitHub releases
   - If no binary available, falls back to building from source (requires Go)
   - Binary is placed in `node_modules/.bin` with a Node.js wrapper

2. **Binary Distribution**:
   - GoReleaser builds binaries for multiple platforms
   - Binaries are attached to GitHub releases
   - Install script downloads the appropriate binary for the user's platform

3. **Execution**:
   - The `bin/river.js` wrapper script forwards all commands to the Go binary
   - Users can run `river`, `river stats`, `river think`, etc.

## Updating the Package

1. Make your code changes
2. Update version in `package.json`
3. Create a new git tag and push
4. Run `goreleaser release --clean`
5. Publish to npm: `npm publish`

## Platform Support

The package supports:
- macOS (Intel and Apple Silicon)
- Linux (x64 and ARM64)  
- Windows (x64)

Users without pre-built binaries can still install if they have Go installed.