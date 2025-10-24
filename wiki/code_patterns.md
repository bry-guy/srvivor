# Code Patterns for srvivor

This document outlines common patterns used in the srvivor codebase to promote decoupling, testability, and maintainability. Agents should reference this when making code changes.

## Interface-Based Dependency Injection

Prefer defining interfaces for dependencies rather than concrete types. This allows for easy mocking in tests and swapping implementations without changing calling code.

### Example: Messager Interface

When adding messaging functionality (e.g., publishing to Discord), define an interface for the sender:

```go
type Messager interface {
    Send(message string) error
}
```

Implement concrete types for each endpoint:

```go
type DiscordMessager struct {
    URL string
}

func NewDiscordMessager(url string) *DiscordMessager {
    return &DiscordMessager{URL: url}
}

func (d *DiscordMessager) Send(message string) error {
    payload := map[string]any{"message": message}
    jsonBytes, err := json.Marshal(payload)
    if err != nil {
        return err
    }
    resp, err := http.Post(d.URL, "application/json", bytes.NewBuffer(jsonBytes))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        slog.Warn("Messager responded with non-200", "status", resp.StatusCode)
    }
    return nil
}
```

Configure and inject the implementation on startup:

```go
func init() {
    cfg, err := config.Validate()
    if err != nil {
        // handle error
    }
    m := messager.NewDiscordMessager(cfg.DiscordBotURL)
    scoreCmd := newScoreCmd(m)
    // add to root
}
```

Use in commands via dependency injection:

```go
func newScoreCmd(m Messager) *cobra.Command {
    return &cobra.Command{
        Run: func(cmd *cobra.Command, args []string) {
            runScore(cmd, args, m)
        },
    }
}

func runScore(cmd *cobra.Command, args []string, m Messager) {
    // use m.Send(message)
}
```

This pattern keeps code decoupled: the score logic doesn't know about HTTP or Discord specifics, only that it can send a message.