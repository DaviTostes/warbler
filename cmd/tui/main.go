package main

import (
	"boteco/internal/config"
	"boteco/internal/db"
	"boteco/internal/gen"
	"fmt"
	"image/color"
	"log/slog"
	"os"
	"strings"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

type errMsg struct{ err error }
type generatedMsg struct {
	resp string
	done bool
	err  error
}

func waitChunk(m model) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.streamCh
		if !ok {
			return nil
		}

		return msg
	}
}

func startGeneration(m model, prompt string) tea.Cmd {
	go func() {
		stream := gen.GenerateStream(m.g, gen.SystemPrompt, prompt, gen.Tools, nil, m.messages)
		for result, err := range stream {
			if err != nil {
				m.streamCh <- generatedMsg{resp: "", err: err}
			}

			if result.Done {
				m.streamCh <- generatedMsg{resp: result.Response.Text(), done: true, err: nil}
				return
			}

			m.streamCh <- generatedMsg{resp: result.Chunk.Text(), err: nil}
		}
	}()

	return waitChunk(m)
}

func WriteMsg(msgs *strings.Builder, m *ai.Message) {
	var c color.Color
	t := "\n  " + m.Text() + "\n\n"

	switch m.Role {
	case "user":
		c = lipgloss.Color("227")
	case "model":
		c = lipgloss.Color("86")
		t, _ = glamour.Render(m.Text(), "dark")
	case "assistant":
		c = lipgloss.Color("3")
	}

	roleStyle := lipgloss.NewStyle().Foreground(c).Bold(true)
	msgs.WriteString(roleStyle.Render("| " + string(m.Role)))
	msgs.WriteString("\n")
	msgs.WriteString(t)
}

type model struct {
	viewport       viewport.Model
	textarea       textarea.Model
	spinner        spinner.Model
	spinning       bool
	g              *genkit.Genkit
	messages       []*ai.Message
	currentMessage string
	streamCh       chan generatedMsg
	generating     bool
	doubleCtrlC    bool
	err            error
}

func (m model) renderMessages() string {
	var b strings.Builder
	for _, msg := range m.messages {
		WriteMsg(&b, msg)
	}
	return lipgloss.NewStyle().
		Width(m.viewport.Width()).
		Height(m.viewport.Height()).
		AlignVertical(lipgloss.Bottom).
		Render(b.String())
}

func (m model) renderLive() string {
	var b strings.Builder
	for _, msg := range m.messages {
		WriteMsg(&b, msg)
	}

	t, _ := glamour.Render(m.currentMessage, "dark")
	t = strings.TrimRight(t, "\r\n")

	roleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)

	b.WriteString(roleStyle.Render("| model"))
	b.WriteString("\n")
	b.WriteString(t)

	return lipgloss.NewStyle().
		Width(m.viewport.Width()).
		Height(m.viewport.Height()).
		AlignVertical(lipgloss.Bottom).
		Render(b.String())
}

func initialModel() model {
	err := db.Connect()
	if err != nil {
		panic(err)
	}

	c, err := config.GetConfig()
	if err != nil {
		panic(err)
	}

	g, err := gen.InitGenkit(c.Gemini.ApiKey)
	if err != nil {
		panic(err)
	}

	ta := textarea.New()
	ta.Placeholder = "Write a message..."
	ta.SetVirtualCursor(false)
	ta.Focus()

	ta.Prompt = "| "
	ta.CharLimit = 10000

	ta.SetWidth(30)
	ta.SetHeight(3)

	s := ta.Styles()
	s.Focused.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(s)

	ta.ShowLineNumbers = false

	vp := viewport.New(viewport.WithWidth(30), viewport.WithHeight(5))
	vp.SetContent(``)
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)

	ta.KeyMap.InsertNewline.SetEnabled(false)

	sp := spinner.New()
	sp.Spinner = spinner.Moon
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		textarea:       ta,
		viewport:       vp,
		spinner:        sp,
		spinning:       false,
		g:              g,
		messages:       []*ai.Message{},
		currentMessage: "",
		streamCh:       make(chan generatedMsg),
		generating:     false,
		err:            nil,
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.SetWidth(msg.Width)
		m.textarea.SetWidth(msg.Width)
		m.viewport.SetHeight(msg.Height - m.textarea.Height())

		if len(m.messages) > 0 {
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, tea.Quit

		case "ctrl+c":
			if m.doubleCtrlC {
				return m, tea.Quit
			}

			content := m.renderMessages() + "\nctrl+c again to quit.\n"
			m.viewport.SetContent(content)
			m.viewport.GotoBottom()

			m.doubleCtrlC = true

		case "pgup", "pgdown", "ctrl+u", "ctrl+d":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd

		case "enter":
			if m.generating {
				return m, nil
			}

			prompt := m.textarea.Value()

			m.messages = append(m.messages, &ai.Message{
				Role: "user",
				Content: []*ai.Part{
					{Text: prompt},
				},
			})

			m.textarea.Reset()

			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()

			m.generating = true
			m.spinning = true

			return m, tea.Batch(startGeneration(m, prompt), m.spinner.Tick)
		}

	case generatedMsg:
		m.spinning = false

		if msg.err != nil {
			m.generating = false
			m.currentMessage = ""
			m.messages = append(m.messages, &ai.Message{
				Role:    "assistant",
				Content: []*ai.Part{{Text: "error: " + msg.err.Error()}},
			})

			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
			return m, nil
		}

		if msg.done {
			m.generating = false
			m.currentMessage = ""
			m.messages = append(m.messages, &ai.Message{
				Role:    "model",
				Content: []*ai.Part{{Text: msg.resp}},
			})

			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
			return m, nil
		}

		m.currentMessage += msg.resp

		m.viewport.SetContent(m.renderLive())
		m.viewport.GotoBottom()

		return m, waitChunk(m)

	case cursor.BlinkMsg:
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd

	case spinner.TickMsg:
		if !m.spinning {
			return m, nil
		}

		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)

		content := m.renderMessages() + "\n " + m.spinner.View() + "\n"
		m.viewport.SetContent(content)
		m.viewport.GotoBottom()
		return m, cmd
	}

	var textareaCmd tea.Cmd
	m.textarea, textareaCmd = m.textarea.Update(msg)
	return m, textareaCmd
}

func (m model) View() tea.View {
	viewportView := m.viewport.View()
	v := tea.NewView(viewportView + "\n" + m.textarea.View())
	v.AltScreen = true

	c := m.textarea.Cursor()
	if c != nil {
		c.Y += lipgloss.Height(viewportView)
	}
	v.Cursor = c
	v.AltScreen = true
	return v
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})))

	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
