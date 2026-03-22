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

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

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
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return model{
		table:   t,
		session: session,
	}
}

func makeColumns(width int) []table.Column {
	// Reserve space for borders (2) and column padding
	usable := width - 4
	if usable < 60 {
		usable = 60
	}

	// Fixed-width columns
	idW := 20
	typeW := 8
	priW := 2
	statusW := 12
	ownerW := 14
	labelsW := 20
	fixed := idW + typeW + priW + statusW + ownerW + labelsW
	titleW := usable - fixed
	if titleW < 10 {
		titleW = 10
	}

	return []table.Column{
		{Title: "ID", Width: idW},
		{Title: "Type", Width: typeW},
		{Title: "P", Width: priW},
		{Title: "Status", Width: statusW},
		{Title: "Owner", Width: ownerW},
		{Title: "Labels", Width: labelsW},
		{Title: "Title", Width: titleW},
	}
}

func beadsToRows(beads []Bead) []table.Row {
	rows := make([]table.Row, len(beads))
	for i, b := range beads {
		rows[i] = table.Row{
			b.ID,
			b.IssueType,
			fmt.Sprintf("%d", b.Priority),
			b.Status,
			b.Owner,
			strings.Join(b.Labels, ", "),
			b.Title,
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
		tableHeight := msg.Height - 6 // header + footer + borders
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
			m.table.SetRows(beadsToRows(msg.beads))
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

	header := fmt.Sprintf(" Rodeo Crush — %s — %d beads", m.session, len(m.beads))
	if !m.lastRefresh.IsZero() {
		header += fmt.Sprintf("  (updated %s)", m.lastRefresh.Format("15:04:05"))
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(header))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf(" Error: %v", m.err)))
		b.WriteString("\n")
	}

	if len(m.beads) == 0 && m.err == nil && !m.lastRefresh.IsZero() {
		b.WriteString(baseStyle.Render(" No beads found. Waiting for agents to create work..."))
		b.WriteString("\n")
	} else {
		b.WriteString(baseStyle.Render(m.table.View()))
		b.WriteString("\n")
	}

	b.WriteString(" q/ctrl-c: quit • ↑↓/j/k: navigate")
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
