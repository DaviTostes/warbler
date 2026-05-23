package main

import (
	"boteco/internal/db"
	"boteco/internal/gen"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

func WriteMsg(msgs *strings.Builder, m *ai.Message) {
	switch m.Role {
	case "user":
		msgs.WriteString("\033[32m")
	case "model":
		msgs.WriteString("\033[35m")
	case "assistant":
		msgs.WriteString("\033[31m")
	}
	msgs.WriteString("| ")
	msgs.WriteString(string(m.Role))
	msgs.WriteString("\033[0m")

	msgs.WriteString("\n")
	msgs.WriteString(" ")
	msgs.WriteString(m.Text())
	msgs.WriteString("\n\n")
}

type errMsg struct{ err error }
type generatedMsg struct {
	resp string
	err  error
}

func generate(m model, prompt string) tea.Cmd {
	return func() tea.Msg {
		resp, err := gen.Generate(m.g, gen.SystemPrompt, prompt, gen.Tools, nil, m.messages)
		return generatedMsg{resp: resp, err: err}
	}
}

type model struct {
	viewport    viewport.Model
	textarea    textarea.Model
	spinner     spinner.Model
	senderStyle lipgloss.Style
	g           *genkit.Genkit
	messages    []*ai.Message
	generating  bool
	err         error
}

func (m model) renderMessages() string {
	var b strings.Builder
	for _, msg := range m.messages {
		WriteMsg(&b, msg)
	}
	return lipgloss.NewStyle().Width(m.viewport.Width()).Render(b.String())
}

func initialModel(g *genkit.Genkit) model {
	ta := textarea.New()
	ta.Placeholder = "Write a message..."
	ta.SetVirtualCursor(false)
	ta.Focus()

	ta.Prompt = "| "
	ta.CharLimit = 200

	ta.SetWidth(30)
	ta.SetHeight(3)

	s := ta.Styles()
	s.Focused.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(s)

	ta.ShowLineNumbers = false

	vp := viewport.New(viewport.WithWidth(30), viewport.WithHeight(5))
	vp.SetContent(`Welcome to boteco!
Type a message and press Enter to send.`)
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)

	ta.KeyMap.InsertNewline.SetEnabled(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		textarea:    ta,
		viewport:    vp,
		spinner:     sp,
		senderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		g:           g,
		messages:    []*ai.Message{},
		generating:  false,
		err:         nil,
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
			m.renderMessages()
		}
		m.viewport.GotoBottom()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			fmt.Println(m.textarea.Value())
			return m, tea.Quit

		case "enter":
			prompt := m.textarea.Value()

			m.messages = append(m.messages, &ai.Message{
				Role: "user",
				Content: []*ai.Part{
					{Text: prompt},
				},
			})

			m.viewport.SetContent(m.renderMessages())
			m.textarea.Reset()
			m.viewport.GotoBottom()

			m.generating = true

			return m, tea.Batch(generate(m, prompt), m.spinner.Tick)

		default:
			var textareaCmd tea.Cmd
			m.textarea, textareaCmd = m.textarea.Update(msg)

			return m, textareaCmd
		}

	case generatedMsg:
		m.generating = false

		if msg.err != nil {
			m.messages = append(m.messages, &ai.Message{
				Role:    "assistant",
				Content: []*ai.Part{{Text: "error: " + msg.err.Error()}},
			})
		} else {
			m.messages = append(m.messages, &ai.Message{
				Role:    "model",
				Content: []*ai.Part{{Text: msg.resp}},
			})
		}

		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)

		return m, cmd

	case cursor.BlinkMsg:
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd

	case spinner.TickMsg:
		if !m.generating {
			return m, nil
		}

		m.viewport.SetContent(m.renderMessages() + "\n " + m.spinner.View() + "Thinking...")

		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() tea.View {
	viewportView := m.viewport.View()
	v := tea.NewView(viewportView + "\n" + m.textarea.View())
	c := m.textarea.Cursor()
	if c != nil {
		c.Y += lipgloss.Height(viewportView)
	}
	v.Cursor = c
	v.AltScreen = true
	return v
}

func main() {
	err := db.Connect()
	if err != nil {
		panic(err)
	}

	g, err := gen.InitGenkit()
	if err != nil {
		panic(err)
	}

	p := tea.NewProgram(initialModel(g))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

}
