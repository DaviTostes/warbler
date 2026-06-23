package main

import (
	"boteco/internal/chat"
	"boteco/internal/db"
	"boteco/internal/gen"
	"fmt"
	"image/color"
	"log/slog"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

type item struct {
	id          uint
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.desc }

var docStyle = lipgloss.NewStyle().Margin(1, 1)

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
		var filteredMessages []*ai.Message
		for _, m := range m.messages {
			if m.Role != "assistant" {
				filteredMessages = append(filteredMessages, m)
			}
		}

		stream := gen.GenerateStream(m.g, gen.BuildSystemPrompt(time.Now()), prompt, gen.Tools, filteredMessages)
		for result, err := range stream {
			if err != nil {
				m.streamCh <- generatedMsg{resp: "", err: err}
				return
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

func waitChatResponse(m model) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.chatCh
		if !ok {
			return nil
		}

		return msg
	}
}

type chatMsg struct {
	id       uint
	title    string
	firstMsg string
	err      error
}

func createChat(m model, firstMessage string) tea.Cmd {
	go func() {
		title, err := gen.Generate(m.g, "Create a chat title based on this message from the user. Return only the title, nothing else", firstMessage, nil, nil)
		if err != nil {
			m.chatCh <- chatMsg{id: 0, title: "", firstMsg: firstMessage, err: err}
		}

		id, err := chat.NewChat(title)
		m.chatCh <- chatMsg{id: id, title: title, firstMsg: firstMessage, err: err}
	}()
	return waitChatResponse(m)
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
	list           list.Model
	mode           string // chat, list
	spinning       bool
	g              *genkit.Genkit
	chatID         uint
	chatTitle      string
	chatCh         chan chatMsg
	messages       []*ai.Message
	currentMessage string
	streamCh       chan generatedMsg
	generating     bool
	doubleCtrlC    bool
	width          int
	height         int
	err            error
}

func (m model) openChat(id uint, title string) (tea.Model, tea.Cmd) {
	msgs, err := chat.GetMessagesFromChat(id)
	if err != nil {
		m.err = err
		return m, nil
	}

	m.messages = m.messages[:0]
	for _, msg := range msgs {
		m.messages = append(m.messages, &ai.Message{
			Role:    ai.Role(msg.Role),
			Content: []*ai.Part{{Text: msg.Text}},
		})
	}

	m.chatID = id
	m.chatTitle = title
	m.mode = "chat"

	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()

	return m, nil
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

func NewModel() model {
	err := db.Connect()
	if err != nil {
		panic(err)
	}

	gen.BuildSystemPrompt(time.Now())

	g, err := gen.InitGenkit()
	if err != nil {
		panic(err)
	}

	ta := textarea.New()
	ta.Placeholder = "Write a message..."
	ta.SetVirtualCursor(false)
	ta.Focus()

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
		mode:           "chat",
		spinning:       false,
		g:              g,
		chatID:         uint(0),
		chatTitle:      "",
		chatCh:         make(chan chatMsg),
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
		m.width = msg.Width
		m.height = msg.Height
		switch m.mode {
		case "list":
			h, v := docStyle.GetFrameSize()
			m.list.SetSize(msg.Width-h, msg.Height-v)

		default:
			headerStyle := lipgloss.NewStyle().
				Bold(true).
				Padding(1, 1)

			header := headerStyle.Render(m.chatTitle)

			headerHeight := lipgloss.Height(header)

			m.viewport.SetWidth(msg.Width)
			m.textarea.SetWidth(msg.Width)
			m.viewport.SetHeight(msg.Height - headerHeight - m.textarea.Height())

			if len(m.messages) > 0 {
				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
			}
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

		case "ctrl+l":
			if m.mode == "chat" {
				m.mode = "list"

				chats, err := chat.GetChats()
				if err != nil {
					panic(err)
				}

				var listItems []list.Item
				for _, c := range chats {
					listItems = append(listItems, item{
						id:    c.ID,
						title: c.Title,
						desc:  c.CreatedAt.String(),
					})
				}

				h, v := docStyle.GetFrameSize()
				m.list = list.New(listItems, list.NewDefaultDelegate(), m.width-h, m.height-v)
				m.list.Title = "Chat History"
			}

		case "pgup", "pgdown", "ctrl+u", "ctrl+d":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd

		case "enter":
			if m.mode == "list" {
				// while filtering, let the list consume enter to apply the filter
				if m.list.FilterState() == list.Filtering {
					break
				}
				if it, ok := m.list.SelectedItem().(item); ok {
					return m.openChat(it.id, it.title)
				}
				return m, nil
			}

			if m.generating {
				return m, nil
			}

			prompt := m.textarea.Value()

			msgs := []tea.Cmd{
				startGeneration(m, prompt),
				m.spinner.Tick,
			}

			if len(m.messages) == 0 {
				msgs = append(msgs, createChat(m, prompt))
			}

			aiMsg := &ai.Message{
				Role: "user",
				Content: []*ai.Part{
					{Text: prompt},
				},
			}

			m.messages = append(m.messages, aiMsg)

			if m.chatID != 0 {
				chat.InsertMessage(m.chatID, aiMsg)
			}

			m.textarea.Reset()

			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()

			m.generating = true
			m.spinning = true

			return m, tea.Batch(msgs...)
		}

	case chatMsg:
		if msg.err != nil {
			m.messages = append(m.messages, &ai.Message{
				Role:    "assistant",
				Content: []*ai.Part{{Text: "error: " + msg.err.Error()}},
			})

			m.spinning = false
			m.generating = false
			m.currentMessage = ""

			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
			return m, nil
		}

		m.chatID = msg.id
		m.chatTitle = msg.title

		chat.InsertMessage(m.chatID, &ai.Message{
			Role: "user",
			Content: []*ai.Part{
				{Text: msg.firstMsg},
			},
		})

	case generatedMsg:
		if msg.err != nil {
			aiMsg := &ai.Message{
				Role:    "assistant",
				Content: []*ai.Part{{Text: "error: " + msg.err.Error()}},
			}

			m.messages = append(m.messages, aiMsg)

			if m.chatID != 0 {
				chat.InsertMessage(m.chatID, aiMsg)
			}

			m.spinning = false
			m.generating = false
			m.currentMessage = ""

			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
			return m, nil
		}

		if msg.done {
			aiMsg := &ai.Message{
				Role:    "model",
				Content: []*ai.Part{{Text: m.currentMessage}},
			}

			m.messages = append(m.messages, aiMsg)

			if m.chatID != 0 {
				chat.InsertMessage(m.chatID, aiMsg)
			}

			m.spinning = false
			m.generating = false
			m.currentMessage = ""

			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
			return m, nil
		}

		m.currentMessage += msg.resp

		if m.currentMessage != "" {
			m.spinning = false
			m.viewport.SetContent(m.renderLive())
			m.viewport.GotoBottom()
		}

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

	switch m.mode {
	case "list":
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	default:
		var textareaCmd tea.Cmd
		m.textarea, textareaCmd = m.textarea.Update(msg)
		return m, textareaCmd
	}
}

func (m model) View() tea.View {
	var v tea.View

	switch m.mode {
	case "list":
		v = tea.NewView(docStyle.Render(m.list.View()))

	default:
		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Padding(1, 1)
		header := headerStyle.Render(m.chatTitle)

		body := lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			m.viewport.View(),
			m.textarea.View(),
		)

		v = tea.NewView(body)

		if c := m.textarea.Cursor(); c != nil {
			c.Y += lipgloss.Height(header) + lipgloss.Height(m.viewport.View())
			v.Cursor = c
		}
	}

	return v
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})))

	p := tea.NewProgram(NewModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
