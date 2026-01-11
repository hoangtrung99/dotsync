# Contributing to Dotsync

Thank you for your interest in contributing to Dotsync!

## Development Setup

```bash
# Clone the repository
git clone https://github.com/yourusername/dotsync.git
cd dotsync

# Install dependencies
go mod download

# Build
make build

# Run tests
make test
```

## Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Add tests for new functionality
- Keep functions focused and small

## Pull Request Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`make test`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## Adding New App Definitions

To add support for a new application, edit `internal/scanner/scanner.go`:

```go
{
    ID:       "myapp",
    Name:     "My App",
    Category: "dev",
    Icon:     "📱",
    ConfigPaths: []string{
        "~/.config/myapp",
        "~/.myapprc",
    },
},
```

### Categories

- `ai` - AI tools (Claude, Copilot)
- `terminal` - Terminal emulators
- `shell` - Shell configurations
- `editor` - Code editors
- `git` - Git tools
- `dev` - Development tools
- `productivity` - Productivity apps
- `cli` - CLI utilities

## Reporting Issues

When reporting issues, please include:

- OS and version
- Go version
- Steps to reproduce
- Expected vs actual behavior

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
