# warbler

Terminal chat assistant in Go. Streams replies right in the terminal. 
Has tool support so it can actually do useful things instead of just making stuff up. 
Conversations are saved to a local SQLite database, so you can reopen past chats.

## tools

Pretty simple. You ask it something, it can call these:

- **Web Search** — scrapes DuckDuckGo. For when you need actual data.
- **Wikipedia** — pulls article summaries straight from the Wikipedia API.
- **Events** — CRUD on a local SQLite calendar. "Remind me about X on Y" actually works.
- **Memories** — persistent memory between sessions. It remembers what you tell it. Novel concept.

## providers

- **Gemini** — needs an API key (Google AI Studio). Model names use the `googleai/` prefix, e.g. `googleai/gemini-3.5-flash`.
- **Ollama** — needs a local Ollama server. Model names use the `ollama/` prefix, e.g. `ollama/llama3.2`.

## prerequisites

- Go 1.26
- A Gemini API key (Google AI Studio), or a local Ollama server

## setup

Put this in `~/.config/warbler/config.json`:

```json
{
  "default": "gemini",
  "gemini": { "api_key": "AIza...", "model": "googleai/gemini-3.5-flash" },
  "ollama": { "server_address": "http://localhost:11434", "model": "ollama/llama3.2" }
}
```

`default` picks the provider: `gemini` or `ollama`. You only need the config
block for the one you use.

## build & use

Run straight from source:

```sh
go run ./cmd/tui
```

Or build a binary and put it on your `PATH`:

```sh
go build -o warbler ./cmd/tui
./warbler
```

## commands

- `Ctrl+L` lists saved chats,
- `PgUp`/`PgDn` or `Ctrl+U`/`Ctrl+D` scroll the chat,
- `Ctrl+C` twice or `Esc` to exit.

---

## why

I wanted a CLI assistant that actually works, doesn't require a browser, and
can call tools. Also, no Electron.
