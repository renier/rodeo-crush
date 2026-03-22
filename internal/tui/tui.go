package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

	sessionStyle = lipgloss.NewStyle().
			Foreground(bright).
			Bold(true)

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

	statusOpen       = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "34", Dark: "78"})
	statusInProgress = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "33", Dark: "75"})
	statusBlocked    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "196", Dark: "203"})
	statusClosed     = lipgloss.NewStyle().Foreground(subtle)
)

type tickMsg time.Time

type beadsMsg struct {
	beads []Bead
	err   error
}

type model struct {
	table       table.Model
	session     string
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
		return statusOpen.Render("○ open")
	case "in_progress":
		return statusInProgress.Render("◐ in_progress")
	case "blocked":
		return statusBlocked.Render("● blocked")
	case "closed":
		return statusClosed.Render("✓ closed")
	default:
		return s
	}
}

func priorityLabel(p int) string {
	switch p {
	case 0:
		return lipgloss.NewStyle().Foreground(warn).Bold(true).Render("P0")
	case 1:
		return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "208", Dark: "214"}).Render("P1")
	case 2:
		return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "33", Dark: "75"}).Render("P2")
	case 3:
		return lipgloss.NewStyle().Foreground(subtle).Render("P3")
	case 4:
		return lipgloss.NewStyle().Foreground(muted).Render("P4")
	default:
		return fmt.Sprintf("P%d", p)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func newModel(session string) model {
	columns := makeColumns(80)
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
		Bold(false)
	s.Cell = s.Cell.
		Foreground(bright)
	t.SetStyles(s)

	return model{
		table:   t,
		session: session,
	}
}

func makeColumns(width int) []table.Column {
	usable := width - 4
	if usable < 60 {
		usable = 60
	}

	idW := 20
	typeW := 8
	priW := 3
	statusW := 14
	labelsW := 22
	descW := 30
	fixed := idW + typeW + priW + statusW + labelsW + descW
	titleW := usable - fixed
	if titleW < 10 {
		titleW = 10
	}

	return []table.Column{
		{Title: "ID", Width: idW},
		{Title: "Type", Width: typeW},
		{Title: "Pri", Width: priW},
		{Title: "Status", Width: statusW},
		{Title: "Labels", Width: labelsW},
		{Title: "Title", Width: titleW},
		{Title: "Description", Width: descW},
	}
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
			strings.Join(b.Labels, ", "),
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
	header := titleStyle.Render("🤠 Rodeo Crush") +
		"  " + sessionStyle.Render(m.session) +
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
func Run(session string) error {
	p := tea.NewProgram(
		newModel(session),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
