# boteco

CLI chat assistant in Go. Uses Genkit + Gemini to stream replies in the
terminal. Has tool support so it can actually do useful things instead of
just making stuff up.

## tools

Pretty simple. You ask it something, it can call these:

- **Web Search** — scrapes DuckDuckGo. For when you need actual data.
- **Events** — CRUD on a local SQLite calendar. "Remind me about X on Y" actually works.
- **Memories** — persistent memory between sessions. It remembers what you tell it. Novel concept.

## prerequisites

- Go 1.26
- A Gemini API key (Google AI Studio)

## setup

Put this in `~/.config/boteco/config.json`:

```json
{ "gemini": { "api_key": "AIza...", "model": "googleai/gemini-3.5-flash" } }
```

## run

```sh
go run cmd/tui/main.go
```

`Ctrl+C` twice or `Esc` to exit.

---

## why

I wanted a CLI assistant that actually works, doesn't require a browser, and
can call tools. Also, no Electron.
