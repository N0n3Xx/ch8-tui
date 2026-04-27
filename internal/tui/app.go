package tui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/N0n3Xx/ch8-tui/internal/config"
	"github.com/N0n3Xx/ch8-tui/internal/ollama"
	"github.com/N0n3Xx/ch8-tui/internal/storage"
	"github.com/N0n3Xx/ch8-tui/internal/telemetry"
)

type screen int

const (
	screenChat screen = iota
	screenModels
	screenChats
)

type genState string

const (
	stateIdle      genState = "idle"
	stateThinking  genState = "thinking"
	stateStreaming genState = "streaming"
	stateDone      genState = "done"
	stateCancelled genState = "cancelled"
	stateError     genState = "error"
	stateStalled   genState = "stalled"
)

type App struct {
	cfg    *config.Config
	store  *storage.Store
	client *ollama.Client

	width  int
	height int

	chat     *storage.Chat
	chats    []*storage.Chat
	models   []ollama.Model
	selected string

	input         textarea.Model
	viewport      viewport.Model
	stickToBottom bool

	screen       screen
	status       genState
	errText      string
	modelErr     string
	modelLoading bool
	telemetryOn  bool

	streaming bool
	stream    <-chan ollama.StreamChunk
	cancel    context.CancelFunc

	startedAt   time.Time
	firstToken  time.Time
	lastToken   time.Time
	completedAt time.Time
	chunkCount  int
	lastTelem   *telemetry.Telemetry

	modelCursor int
	chatCursor  int
	chatFilter  string
	confirmDel  bool
	editingLast bool
}

type modelsLoadedMsg struct {
	models []ollama.Model
	err    error
}

type chatsLoadedMsg struct {
	chats []*storage.Chat
	err   error
}

type streamMsg ollama.StreamChunk
type saveDoneMsg struct{ err error }
type tickMsg time.Time
type resetStateMsg struct{}

type footerItem struct {
	key   string
	label string
	sep   bool
}

func New(cfg *config.Config, store *storage.Store, client *ollama.Client) *App {
	input := textarea.New()
	input.Placeholder = ""
	input.Prompt = ""
	input.CharLimit = 20000
	input.ShowLineNumbers = false
	input.SetHeight(3)
	input.Focus()

	model := cfg.DefaultModel
	return &App{
		cfg:           cfg,
		store:         store,
		client:        client,
		chat:          storage.NewChat(model),
		selected:      model,
		input:         input,
		viewport:      viewport.New(80, 20),
		stickToBottom: true,
		status:        stateIdle,
		modelLoading:  true,
		telemetryOn:   cfg.TelemetryEnabled,
	}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(loadModels(a.client), loadChats(a.store), tick())
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.resize()
		a.refreshViewport(false)
	case modelsLoadedMsg:
		a.modelLoading = false
		if msg.err != nil {
			a.modelErr = msg.err.Error()
			a.errText = msg.err.Error()
			if !a.streaming {
				a.status = stateError
			}
			return a, nil
		}
		a.modelErr = ""
		a.models = msg.models
		if a.selected == "" && len(a.models) > 0 {
			a.selected = a.models[0].Name
			a.chat.SelectedModel = a.selected
		}
		for i, model := range a.models {
			if model.Name == a.selected {
				a.modelCursor = i
				break
			}
		}
		if !a.streaming && a.status == stateError {
			a.status = stateIdle
		}
	case chatsLoadedMsg:
		if msg.err != nil {
			a.errText = msg.err.Error()
		} else {
			a.chats = msg.chats
		}
	case streamMsg:
		cmd := a.handleStream(ollama.StreamChunk(msg))
		a.refreshViewport(false)
		return a, cmd
	case saveDoneMsg:
		if msg.err != nil {
			a.errText = "save failed: " + msg.err.Error()
		}
	case tickMsg:
		a.updateLifecycle(time.Time(msg))
		return a, tick()
	case resetStateMsg:
		if !a.streaming && a.status == stateDone {
			a.status = stateIdle
		}
	case tea.KeyMsg:
		if a.screen != screenChat || isChatCommandKey(msg) {
			cmd := a.handleKey(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return a, tea.Batch(cmds...)
		}
	}

	if a.screen == screenChat {
		var cmd tea.Cmd
		a.input, cmd = a.input.Update(msg)
		cmds = append(cmds, cmd)
		a.resize()
		a.viewport, cmd = a.viewport.Update(msg)
		a.stickToBottom = a.viewport.AtBottom()
		cmds = append(cmds, cmd)
	}
	return a, tea.Batch(cmds...)
}

func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "Starting..."
	}
	sections := []string{a.chatPanel()}
	if a.telemetryOn {
		sections = append(sections, a.telemetryPanel())
	}
	sections = append(sections, a.inputPanel(), a.footer())
	base := lipgloss.JoinVertical(lipgloss.Left, sections...)
	switch a.screen {
	case screenModels:
		return overlay(a.width, base, a.modelSelector())
	case screenChats:
		return overlay(a.width, base, a.chatBrowser())
	default:
		return base
	}
}

func (a *App) handleKey(key tea.KeyMsg) tea.Cmd {
	switch a.screen {
	case screenModels:
		return a.handleModelKeys(key)
	case screenChats:
		return a.handleChatKeys(key)
	}

	switch key.String() {
	case "ctrl+c":
		if a.streaming {
			a.cancelStream()
			return nil
		}
		return tea.Quit
	case "enter":
		if a.streaming {
			return nil
		}
		if strings.TrimSpace(a.input.Value()) == "" {
			return a.openModelSelector()
		}
		return a.sendInput()
	case "alt+enter", "shift+enter":
		a.input.SetValue(a.input.Value() + "\n")
	case "ctrl+n":
		a.cancelStream()
		a.chat = storage.NewChat(a.selected)
		a.stickToBottom = true
		a.lastTelem = nil
		a.errText = ""
		a.status = stateIdle
		a.input.SetValue("")
		a.refreshViewport(true)
	case "ctrl+s":
		return saveChat(a.store, a.chat)
	case "ctrl+o", "alt+o", "f3":
		return a.openChatBrowser()
	case "ctrl+m", "alt+m", "f2":
		return a.openModelSelector()
	case "ctrl+t", "alt+t", "f4":
		a.toggleTelemetry()
	case "ctrl+r":
		if !a.streaming {
			return a.regenerate()
		}
	case "ctrl+e":
		if !a.streaming {
			a.editLastUser()
			a.refreshViewport(true)
		}
	case "ctrl+l":
		a.errText = ""
	case "end", "ctrl+j":
		a.stickToBottom = true
		a.viewport.GotoBottom()
	}
	return nil
}

func isChatCommandKey(key tea.KeyMsg) bool {
	switch key.String() {
	case "ctrl+c", "enter", "alt+enter", "shift+enter", "ctrl+n", "ctrl+s", "ctrl+o", "ctrl+m", "ctrl+t", "ctrl+r", "ctrl+e", "ctrl+l", "ctrl+j", "end", "alt+m", "alt+o", "alt+t", "f2", "f3", "f4":
		return true
	default:
		return false
	}
}

func (a *App) openModelSelector() tea.Cmd {
	a.screen = screenModels
	a.modelErr = ""
	a.modelLoading = true
	return loadModels(a.client)
}

func (a *App) openChatBrowser() tea.Cmd {
	a.screen = screenChats
	a.chatFilter = ""
	a.confirmDel = false
	return loadChats(a.store)
}

func (a *App) toggleTelemetry() {
	a.telemetryOn = !a.telemetryOn
	a.cfg.TelemetryEnabled = a.telemetryOn
	_ = config.Save(a.cfg)
	a.resize()
	a.refreshViewport(false)
}

func (a *App) handleModelKeys(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "esc", "ctrl+c":
		a.screen = screenChat
	case "up", "k":
		if a.modelCursor > 0 {
			a.modelCursor--
		}
	case "down", "j":
		if a.modelCursor < len(a.models)-1 {
			a.modelCursor++
		}
	case "enter":
		if len(a.models) > 0 {
			a.selected = a.models[a.modelCursor].Name
			a.chat.SelectedModel = a.selected
			a.cfg.DefaultModel = a.selected
			_ = config.Save(a.cfg)
			a.screen = screenChat
			a.errText = ""
			if !a.streaming {
				a.status = stateIdle
			}
		}
	case "r":
		a.modelErr = ""
		a.modelLoading = true
		return loadModels(a.client)
	}
	return nil
}

func (a *App) handleChatKeys(key tea.KeyMsg) tea.Cmd {
	if a.confirmDel {
		switch key.String() {
		case "y":
			chats := a.filteredChats()
			if len(chats) > 0 {
				id := chats[a.chatCursor].ID
				a.confirmDel = false
				return deleteAndLoadChats(a.store, id)
			}
		case "n", "esc":
			a.confirmDel = false
		}
		return nil
	}
	switch key.String() {
	case "esc", "ctrl+c":
		a.screen = screenChat
	case "up", "k":
		if a.chatCursor > 0 {
			a.chatCursor--
		}
	case "down", "j":
		if a.chatCursor < len(a.filteredChats())-1 {
			a.chatCursor++
		}
	case "enter":
		chats := a.filteredChats()
		if len(chats) > 0 {
			chat, err := a.store.Load(chats[a.chatCursor].ID)
			if err != nil {
				a.errText = err.Error()
				return nil
			}
			a.chat = chat
			a.selected = chat.SelectedModel
			a.lastTelem = lastTelemetry(chat)
			a.input.SetValue("")
			a.screen = screenChat
			a.stickToBottom = true
			a.refreshViewport(true)
		}
	case "d":
		if len(a.filteredChats()) > 0 {
			a.confirmDel = true
		}
	case "backspace":
		if len(a.chatFilter) > 0 {
			a.chatFilter = a.chatFilter[:len(a.chatFilter)-1]
			a.chatCursor = 0
		}
	default:
		if len(key.String()) == 1 {
			a.chatFilter += key.String()
			a.chatCursor = 0
		}
	}
	return nil
}

func (a *App) sendInput() tea.Cmd {
	content := strings.TrimSpace(a.input.Value())
	if content == "" {
		return nil
	}
	if a.selected == "" {
		a.errText = "No model selected. Press F2 or Alt+M to choose an installed Ollama model."
		return nil
	}
	if a.editingLast {
		a.removeAfterLastUser()
		a.editingLast = false
	}
	a.chat.SelectedModel = a.selected
	a.chat.Messages = append(a.chat.Messages, storage.Message{Role: "user", Content: content, Timestamp: time.Now()})
	a.chat.Title = storage.TitleFromMessages(a.chat.Messages)
	a.input.SetValue("")
	a.errText = ""
	a.startStream()
	a.refreshViewport(true)
	return tea.Batch(saveChat(a.store, a.chat), readStream(a.stream))
}

func (a *App) regenerate() tea.Cmd {
	for len(a.chat.Messages) > 0 && a.chat.Messages[len(a.chat.Messages)-1].Role == "assistant" {
		a.chat.Messages = a.chat.Messages[:len(a.chat.Messages)-1]
	}
	if len(a.chat.Messages) == 0 || a.chat.Messages[len(a.chat.Messages)-1].Role != "user" {
		return nil
	}
	a.startStream()
	a.refreshViewport(true)
	return readStream(a.stream)
}

func (a *App) startStream() {
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.streaming = true
	a.startedAt = time.Now()
	a.firstToken = time.Time{}
	a.lastToken = time.Time{}
	a.completedAt = time.Time{}
	a.chunkCount = 0
	a.status = stateThinking
	requestMessages := a.contextMessages()
	a.chat.Messages = append(a.chat.Messages, storage.Message{
		Role:      "assistant",
		Timestamp: time.Now(),
		Model:     a.selected,
	})
	a.stream = a.client.StreamChat(ctx, a.selected, a.cfg.DefaultSystemPrompt, requestMessages)
}

func (a *App) handleStream(chunk ollama.StreamChunk) tea.Cmd {
	if chunk.Err != nil {
		if errors.Is(chunk.Err, context.Canceled) {
			a.status = stateCancelled
		} else {
			a.status = stateError
			a.errText = chunk.Err.Error()
		}
		a.streaming = false
		a.completedAt = time.Now()
		return saveChat(a.store, a.chat)
	}
	if chunk.Content != "" && len(a.chat.Messages) > 0 {
		now := time.Now()
		if a.firstToken.IsZero() {
			a.firstToken = now
		}
		a.lastToken = now
		a.chunkCount++
		a.status = stateStreaming
		a.chat.Messages[len(a.chat.Messages)-1].Content += chunk.Content
	}
	if chunk.Done {
		a.streaming = false
		a.status = stateDone
		a.completedAt = time.Now()
		a.lastTelem = chunk.Telemetry
		if chunk.Telemetry != nil && len(a.chat.Messages) > 0 {
			a.chat.Messages[len(a.chat.Messages)-1].Telemetry = chunk.Telemetry
		}
		return tea.Batch(saveChat(a.store, a.chat), resetStateAfter(2500*time.Millisecond))
	}
	return readStream(a.stream)
}

func (a *App) cancelStream() {
	if a.cancel != nil {
		a.cancel()
	}
	a.streaming = false
	a.status = stateCancelled
	a.completedAt = time.Now()
}

func (a *App) editLastUser() {
	for i := len(a.chat.Messages) - 1; i >= 0; i-- {
		if a.chat.Messages[i].Role == "user" {
			a.input.SetValue(a.chat.Messages[i].Content)
			a.chat.Messages = a.chat.Messages[:i]
			a.editingLast = true
			return
		}
	}
}

func (a *App) removeAfterLastUser() {
	for i := len(a.chat.Messages) - 1; i >= 0; i-- {
		if a.chat.Messages[i].Role == "user" {
			a.chat.Messages = a.chat.Messages[:i]
			return
		}
	}
}

func (a *App) contextMessages() []storage.Message {
	msgs := a.chat.Messages
	if a.cfg.MaxContextMessages > 0 && len(msgs) > a.cfg.MaxContextMessages {
		msgs = msgs[len(msgs)-a.cfg.MaxContextMessages:]
	}
	if a.cfg.MaxContextChars <= 0 {
		return msgs
	}
	total := 0
	start := len(msgs)
	for i := len(msgs) - 1; i >= 0; i-- {
		total += len(msgs[i].Content)
		if total > a.cfg.MaxContextChars {
			break
		}
		start = i
	}
	return msgs[start:]
}

func (a *App) resize() {
	telemetryHeight := 0
	if a.telemetryOn {
		telemetryHeight = 4
	}
	inputHeight := a.inputHeight()
	footerHeight := 1
	inputPanelHeight := inputHeight + 3
	chatPanelChrome := 3
	chatHeight := max(3, a.height-footerHeight-telemetryHeight-inputPanelHeight-chatPanelChrome)
	a.viewport.Width = max(10, a.width-6)
	a.viewport.Height = chatHeight
	a.input.SetWidth(max(10, a.width-8))
	a.input.SetHeight(inputHeight)
}

func (a *App) refreshViewport(forceBottom bool) {
	content := a.renderMessages()
	if contentHeight := lipgloss.Height(content); contentHeight < a.viewport.Height {
		content = strings.Repeat("\n", a.viewport.Height-contentHeight) + content
	}
	a.viewport.SetContent(content)
	if forceBottom || a.stickToBottom {
		a.viewport.GotoBottom()
		a.stickToBottom = true
	}
}

func (a *App) chatPanel() string {
	title := truncate(safeDisplayText(a.chat.Title), max(12, a.width/3))
	model := safeDisplayText(emptyDefault(a.selected, "none"))
	header := fitPair(" chat: "+title+" ", fmt.Sprintf("model: %s  state: %s  %s", model, a.statePlain(), a.elapsedLabel()), max(10, a.width-4))
	return panelStyle.Width(max(0, a.width-2)).Render(labelStyle.Render(header) + "\n" + a.viewport.View())
}

func (a *App) telemetryPanel() string {
	lines := []string{labelStyle.Render(" telemetry ")}
	if a.streaming {
		first := "pending"
		if !a.firstToken.IsZero() {
			first = a.firstToken.Sub(a.startedAt).Round(time.Millisecond).String()
		}
		lines = append(lines, fmt.Sprintf("state %s | elapsed %s | first token %s | last token %s | chunks %d", a.statePlain(), time.Since(a.startedAt).Round(100*time.Millisecond), first, a.lastTokenAgo(), a.chunkCount))
	} else if a.lastTelem != nil {
		t := a.lastTelem
		lines = append(lines,
			fmt.Sprintf("response %s | first token %s | tok/s %.1f | prompt %d | completion %d | total %d", t.ResponseTime().Round(time.Millisecond), t.FirstTokenTime().Round(time.Millisecond), t.TokensPerSecond, t.PromptTokens, t.CompletionTokens, t.TotalTokens),
			fmt.Sprintf("load %s | prompt eval %s | generation %s | model %s", durNanos(t.LoadDurationNanos), durNanos(t.PromptEvalDurationNanos), durNanos(t.EvalDurationNanos), safeDisplayText(t.Model)),
		)
	} else {
		lines = append(lines, fmt.Sprintf("state %s | model %s | chat %s", a.statePlain(), safeDisplayText(emptyDefault(a.selected, "none")), safeDisplayText(a.chat.ID)))
	}
	return panelStyle.Width(max(0, a.width-2)).Render(strings.Join(lines, "\n"))
}

func (a *App) inputPanel() string {
	label := " input "
	if strings.TrimSpace(a.input.Value()) == "" {
		label = " input - type a message "
	}
	return inputStyle.Width(max(0, a.width-2)).Render(labelStyle.Render(label) + "\n> " + a.input.View())
}

func (a *App) footer() string {
	width := max(1, a.width)
	items := a.footerItems(width)
	lineWidth := footerItemsWidth(items)
	extra := a.footerExtra()
	if extra != "" {
		remaining := width - lineWidth - 3
		if remaining >= 10 {
			items = append(items, footerItem{sep: true, label: truncate(extra, remaining)})
		}
	}
	return statusStyle.Render(renderFooterItems(items, width))
}

func (a *App) footerExtra() string {
	if a.errText != "" {
		return safeDisplayText(a.errText)
	}
	if a.lastTelem != nil && !a.streaming {
		return fmt.Sprintf("tok/s %.1f | tokens %d", a.lastTelem.TokensPerSecond, a.lastTelem.TotalTokens)
	}
	if a.streaming {
		return a.lastTokenAgo()
	}
	return ""
}

func (a *App) renderMessages() string {
	if len(a.chat.Messages) == 0 {
		return emptyStyle.Width(max(10, a.width-8)).Height(max(1, a.viewport.Height)).Render("No messages yet. Press F2 or Alt+M to choose a model, then type a message.")
	}
	var b strings.Builder
	for _, msg := range a.chat.Messages {
		if msg.Role == "system" {
			continue
		}
		header := strings.ToUpper(msg.Role)
		if msg.Model != "" {
			header += " - " + msg.Model
		}
		style := userStyle
		if msg.Role == "assistant" {
			style = assistantStyle
		}
		fmt.Fprintln(&b, style.Render(header))
		content := strings.TrimRight(msg.Content, "\n")
		if content == "" && msg.Role == "assistant" && a.streaming {
			content = "..."
		}
		content = safeDisplayText(content)
		if msg.Role == "assistant" {
			fmt.Fprintln(&b, renderAssistantMarkdown(content, max(20, a.width-10)))
		} else {
			fmt.Fprintln(&b, wrapStyle.Width(max(20, a.width-10)).Render(content))
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

func (a *App) modelSelector() string {
	var b strings.Builder
	fmt.Fprintln(&b, labelStyle.Render(" models "))
	if a.modelLoading {
		fmt.Fprintln(&b, "Fetching models from Ollama...")
	} else if a.modelErr != "" {
		fmt.Fprintf(&b, "Ollama unreachable\n%s\n\nStart Ollama or check ollama_base_url, then press r.\n", safeDisplayText(a.modelErr))
	} else if len(a.models) == 0 {
		fmt.Fprintln(&b, "No installed models found. Start Ollama and pull a model, then press r.")
	} else {
		for i, model := range a.models {
			cursor := "  "
			if i == a.modelCursor {
				cursor = "> "
			}
			marker := " "
			if model.Name == a.selected {
				marker = "*"
			}
			fmt.Fprintf(&b, "%s%s %-34s %9s  %s\n", cursor, marker, truncate(safeDisplayText(model.Name), 34), humanSize(model.Size), model.ModifiedAt.Format("2006-01-02"))
		}
	}
	fmt.Fprintln(&b, "\nEnter select | r refresh | Esc close")
	return modalStyle.Width(min(82, a.width-4)).Render(b.String())
}

func (a *App) chatBrowser() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s /%s\n", labelStyle.Render(" chats "), safeDisplayText(a.chatFilter))
	chats := a.filteredChats()
	if len(chats) == 0 {
		fmt.Fprintln(&b, "No saved chats match.")
	} else {
		for i, chat := range chats {
			cursor := "  "
			if i == a.chatCursor {
				cursor = "> "
			}
			fmt.Fprintf(&b, "%s%-32s %-18s %s\n    %s\n", cursor, truncate(safeDisplayText(chat.Title), 32), truncate(safeDisplayText(chat.SelectedModel), 18), chat.UpdatedAt.Format("Jan 02 15:04"), safeDisplayText(storage.Preview(chat)))
		}
	}
	if a.confirmDel {
		fmt.Fprintln(&b, "\nDelete selected chat? y/n")
	} else {
		fmt.Fprintln(&b, "\nType to filter | Enter open | d delete | Esc close")
	}
	return modalStyle.Width(min(90, a.width-4)).Render(b.String())
}

func (a *App) filteredChats() []*storage.Chat {
	if a.chatFilter == "" {
		return a.chats
	}
	q := strings.ToLower(a.chatFilter)
	out := []*storage.Chat{}
	for _, chat := range a.chats {
		hay := strings.ToLower(chat.Title + " " + chat.SelectedModel + " " + storage.Preview(chat))
		if strings.Contains(hay, q) {
			out = append(out, chat)
		}
	}
	if a.chatCursor >= len(out) {
		a.chatCursor = max(0, len(out)-1)
	}
	return out
}

func (a *App) updateLifecycle(now time.Time) {
	if !a.streaming {
		return
	}
	base := a.startedAt
	if !a.lastToken.IsZero() {
		base = a.lastToken
	}
	threshold := time.Duration(a.cfg.StallThresholdSecs) * time.Second
	if threshold <= 0 {
		threshold = 10 * time.Second
	}
	if now.Sub(base) >= threshold {
		a.status = stateStalled
	}
}

func (a *App) statePlain() string {
	symbol := map[genState]string{
		stateIdle:      "o",
		stateThinking:  "~",
		stateStreaming: ">",
		stateDone:      "ok",
		stateCancelled: "x",
		stateError:     "!",
		stateStalled:   "...",
	}[a.status]
	if symbol == "" {
		symbol = "o"
	}
	return fmt.Sprintf("%s %s", symbol, a.status)
}

func (a *App) elapsedLabel() string {
	if a.startedAt.IsZero() {
		return ""
	}
	if a.streaming {
		return "elapsed: " + time.Since(a.startedAt).Round(100*time.Millisecond).String()
	}
	if !a.completedAt.IsZero() {
		return "elapsed: " + a.completedAt.Sub(a.startedAt).Round(time.Millisecond).String()
	}
	return ""
}

func (a *App) lastTokenAgo() string {
	if a.lastToken.IsZero() {
		if a.streaming && !a.startedAt.IsZero() {
			return "last token " + time.Since(a.startedAt).Round(100*time.Millisecond).String() + " ago"
		}
		return "last token n/a"
	}
	return "last token " + time.Since(a.lastToken).Round(100*time.Millisecond).String() + " ago"
}

func (a *App) inputHeight() int {
	lines := strings.Count(a.input.Value(), "\n") + 3
	return min(6, max(3, lines))
}

func (a *App) footerItems(width int) []footerItem {
	variants := [][]footerItem{
		{
			{key: "F2/Alt+M", label: "Models"},
			{key: "F3/Alt+O", label: "Chats"},
			{key: "F4/Alt+T", label: "Telemetry"},
			{key: "Ctrl+N", label: "New"},
			{key: "Ctrl+R", label: "Regen"},
			{key: "Ctrl+C", label: "Quit"},
		},
		{
			{key: "F2", label: "Models"},
			{key: "F3", label: "Chats"},
			{key: "F4", label: "Telemetry"},
			{key: "^N", label: "New"},
			{key: "^R", label: "Regen"},
			{key: "^C", label: "Quit"},
		},
		{
			{key: "F2", label: "Models"},
			{key: "F3", label: "Chats"},
			{key: "F4", label: "Stats"},
			{key: "^N", label: "New"},
			{key: "^R", label: "Retry"},
			{key: "^C", label: "Quit"},
		},
		{
			{key: "F2", label: "Models"},
			{key: "F3", label: "Chats"},
			{key: "F4", label: "Stats"},
			{key: "^C", label: "Quit"},
		},
		{
			{key: "F2", label: "Model"},
			{key: "F3", label: "Chats"},
			{key: "^C", label: "Quit"},
		},
	}
	for _, items := range variants {
		if footerItemsWidth(items) <= width {
			return items
		}
	}
	return variants[len(variants)-1]
}

func footerItemsWidth(items []footerItem) int {
	width := 0
	for i, item := range items {
		if i > 0 || item.sep {
			width += 3
		}
		if item.sep {
			width += lipgloss.Width(item.label)
			continue
		}
		width += lipgloss.Width(item.key)
		if item.label != "" {
			width += 1 + lipgloss.Width(item.label)
		}
	}
	return width
}

func renderFooterItems(items []footerItem, width int) string {
	var b strings.Builder
	used := 0
	for i, item := range items {
		if i > 0 || item.sep {
			if used+3 > width {
				break
			}
			b.WriteString(sepStyle.Render(" | "))
			used += 3
		}
		if item.sep {
			label := truncate(item.label, width-used)
			b.WriteString(label)
			used += lipgloss.Width(label)
			continue
		}
		plain := item.key
		if item.label != "" {
			plain += " " + item.label
		}
		if used+lipgloss.Width(plain) > width {
			remaining := width - used
			if remaining > 0 {
				part := truncate(plain, remaining)
				b.WriteString(part)
				used += lipgloss.Width(part)
			}
			break
		}
		b.WriteString(keyStyle.Render(item.key))
		if item.label != "" {
			b.WriteString(" " + item.label)
		}
		used += lipgloss.Width(plain)
	}
	if used < width {
		b.WriteString(strings.Repeat(" ", width-used))
	}
	return b.String()
}

func loadModels(client *ollama.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		models, err := client.ListModels(ctx)
		return modelsLoadedMsg{models: models, err: err}
	}
}

func loadChats(store *storage.Store) tea.Cmd {
	return func() tea.Msg {
		chats, err := store.List()
		return chatsLoadedMsg{chats: chats, err: err}
	}
}

func saveChat(store *storage.Store, chat *storage.Chat) tea.Cmd {
	snapshot := cloneChat(chat)
	return func() tea.Msg {
		return saveDoneMsg{err: store.Save(snapshot)}
	}
}

func cloneChat(chat *storage.Chat) *storage.Chat {
	if chat == nil {
		return nil
	}
	cp := *chat
	cp.Messages = append([]storage.Message(nil), chat.Messages...)
	return &cp
}

func deleteAndLoadChats(store *storage.Store, id string) tea.Cmd {
	return func() tea.Msg {
		if err := store.Delete(id); err != nil {
			return saveDoneMsg{err: err}
		}
		chats, err := store.List()
		return chatsLoadedMsg{chats: chats, err: err}
	}
}

func readStream(ch <-chan ollama.StreamChunk) tea.Cmd {
	return func() tea.Msg {
		chunk, ok := <-ch
		if !ok {
			return streamMsg{Done: true}
		}
		return streamMsg(chunk)
	}
}

func tick() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func resetStateAfter(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return resetStateMsg{}
	})
}

func lastTelemetry(chat *storage.Chat) *telemetry.Telemetry {
	for i := len(chat.Messages) - 1; i >= 0; i-- {
		if chat.Messages[i].Telemetry != nil {
			return chat.Messages[i].Telemetry
		}
	}
	return nil
}

func overlay(width int, base, modal string) string {
	return lipgloss.Place(width, lipgloss.Height(base), lipgloss.Center, lipgloss.Center, modal, lipgloss.WithWhitespaceChars(" "))
}

func durNanos(n int64) string {
	if n <= 0 {
		return "n/a"
	}
	return time.Duration(n).Round(time.Millisecond).String()
}

func humanSize(n int64) string {
	if n <= 0 {
		return "unknown"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	return fmt.Sprintf("%.1f%s", v, units[i])
}

func fitPair(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	left = truncate(left, width)
	remaining := width - lipgloss.Width(left)
	if remaining <= 1 {
		return left
	}
	right = truncate(right, remaining)
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

func emptyDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	return int(math.Max(float64(a), float64(b)))
}
