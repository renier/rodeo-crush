package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/renier/rodeo-crush/internal/config"
)

const refreshInterval = 5 * time.Second

var (
	subtle = lipgloss.AdaptiveColor{Light: "245", Dark: "241"}
	accent = lipgloss.AdaptiveColor{Light: "99", Dark: "105"}
	bright = lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
	warn   = lipgloss.AdaptiveColor{Light: "196", Dark: "203"}
	muted  = lipgloss.AdaptiveColor{Light: "250", Dark: "238"}

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			PaddingLeft(1)

	countStyle = lipgloss.NewStyle().
			Foreground(subtle)

	timestampStyle = lipgloss.NewStyle().
			Foreground(subtle).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(warn).
			Bold(true).
			PaddingLeft(1)

	emptyStyle = lipgloss.NewStyle().
			Foreground(subtle).
			Italic(true).
			PaddingLeft(2).
			PaddingTop(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(subtle).
			PaddingLeft(1)

	tableStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(subtle)
)

type tickMsg time.Time

type beadsMsg struct {
	beads []Bead
	err   error
}

type model struct {
	table       table.Model
	styles      table.Styles
	beads       []Bead
	lastRefresh time.Time
	err         error
	width       int
	height      int
	quitting    bool
}

func statusIcon(s string) string {
	switch s {
	case "open":
		return "○ open"
	case "in_progress":
		return "◐ in progress"
	case "blocked":
		return "● blocked"
	case "closed":
		return "✓ closed"
	default:
		return s
	}
}

func priorityLabel(p int) string {
	switch p {
	case 0:
		return "🔴 P0"
	case 1:
		return "🟠 P1"
	case 2:
		return "🔵 P2"
	case 3:
		return "⚪ P3"
	case 4:
		return "⚫ P4"
	default:
		return fmt.Sprintf("P%d", p)
	}
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func newModel() model {
	initWidth := 80
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
		initWidth = w
	}
	columns := makeColumns(initWidth)
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(nil),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(accent).
		BorderBottom(true).
		Bold(true).
		Foreground(bright)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.AdaptiveColor{Light: "99", Dark: "57"}).
		Bold(false).
		Width(columnsWidth(columns))
	t.SetStyles(s)

	return model{
		table:  t,
		styles: s,
	}
}

type columnDef struct {
	title   string
	percent float64
	minW    int
}

var columnDefs = []columnDef{
	{title: "ID", percent: 0.10, minW: 8},
	{title: "Type", percent: 0.06, minW: 5},
	{title: "Priority", percent: 0.04, minW: 5},
	{title: "Status", percent: 0.06, minW: 10},
	{title: "Role", percent: 0.05, minW: 8},
	{title: "Title", percent: 0.25, minW: 10},
	{title: "Description", percent: 0.25, minW: 10},
}

func columnsWidth(cols []table.Column) int {
	w := 0
	for _, c := range cols {
		w += c.Width
	}
	return w
}

func makeColumns(width int) []table.Column {
	usable := width - 4
	if usable < 60 {
		usable = 60
	}

	cols := make([]table.Column, len(columnDefs))
	remaining := usable

	for i, def := range columnDefs {
		w := int(float64(usable) * def.percent)
		if w < def.minW {
			w = def.minW
		}
		cols[i] = table.Column{Title: def.title, Width: w}
		remaining -= w
	}

	if remaining > 0 {
		cols[len(cols)-2].Width += max(0, remaining-12)
	}

	return cols
}

func beadsToRows(beads []Bead, columns []table.Column) []table.Row {
	rows := make([]table.Row, len(beads))
	descW := columns[6].Width
	for i, b := range beads {
		desc := strings.ReplaceAll(b.Description, "\n", " ")
		desc = truncate(desc, descW)
		rows[i] = table.Row{
			b.ID,
			b.IssueType,
			priorityLabel(b.Priority),
			statusIcon(b.Status),
			b.Assignee,
			b.Title,
			desc,
		}
	}
	return rows
}

func fetchBeadsCmd() tea.Msg {
	beads, err := FetchBeads()
	return beadsMsg{beads: beads, err: err}
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(fetchBeadsCmd, tickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		columns := makeColumns(msg.Width)
		m.table.SetColumns(columns)
		m.table.SetRows(beadsToRows(m.beads, columns))
		m.styles.Selected = m.styles.Selected.Width(columnsWidth(columns))
		m.table.SetStyles(m.styles)
		tableHeight := msg.Height - 6
		if tableHeight < 3 {
			tableHeight = 3
		}
		m.table.SetHeight(tableHeight)

	case beadsMsg:
		m.lastRefresh = time.Now()
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.err = nil
			m.beads = msg.beads
			m.table.SetRows(beadsToRows(msg.beads, m.table.Columns()))
		}

	case tickMsg:
		return m, tea.Batch(fetchBeadsCmd, tickCmd())
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Header
	header := titleStyle.Render(config.AppName) +
		"  " + countStyle.Render(fmt.Sprintf("%d beads", len(m.beads)))
	if !m.lastRefresh.IsZero() {
		header += "  " + timestampStyle.Render(m.lastRefresh.Format("15:04:05"))
	}
	b.WriteString(header)
	b.WriteString("\n\n")

	// Error
	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("✗ %v", m.err)))
		b.WriteString("\n\n")
	}

	// Table or empty state
	if len(m.beads) == 0 && m.err == nil && !m.lastRefresh.IsZero() {
		b.WriteString(emptyStyle.Render("No beads found. Waiting for agents to create work..."))
		b.WriteString("\n")
	} else {
		b.WriteString(tableStyle.Render(m.table.View()))
		b.WriteString("\n")
	}

	// Footer
	b.WriteString(helpStyle.Render("q quit • ↑↓ navigate • pgup/pgdn scroll"))
	return b.String()
}

// Run starts the Bubble Tea TUI. It blocks until the user quits.
func Run() error {
	p := tea.NewProgram(
		newModel(),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
