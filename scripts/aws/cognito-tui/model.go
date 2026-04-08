package main

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen identifies the active TUI screen.
type Screen int

const (
	ScreenLoading    Screen = iota // initial + all async transitions
	ScreenProfiles                 // AWS profile selection
	ScreenPools                    // user pool selection
	ScreenUsers                    // users table
	ScreenUserDetail               // single-user detail + actions
	ScreenConfirm                  // y/n confirmation dialog
	ScreenError                    // fatal error
)

// Action identifies the pending destructive operation.
type Action int

const (
	ActionNone Action = iota
	ActionEnable
	ActionDisable
	ActionDelete
	ActionResetPassword
	ActionConfirmUser
	ActionSignOut
)

// ── List item types ───────────────────────────────────────────────────────────

type profileItem string

func (p profileItem) FilterValue() string { return string(p) }
func (p profileItem) Title() string       { return string(p) }
func (p profileItem) Description() string {
	if r := GetProfileRegion(string(p)); r != "" {
		return "region: " + r
	}
	return "region: (default)"
}

type poolListItem struct{ pool PoolItem }

func (p poolListItem) FilterValue() string { return p.pool.Name + " " + p.pool.ID }
func (p poolListItem) Title() string       { return p.pool.Name }
func (p poolListItem) Description() string { return p.pool.ID }

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	screen int // Screen
	width  int
	height int

	// Loading
	spinner    spinner.Model
	loadingMsg string

	// Profile
	profileList     list.Model
	selectedProfile string

	// AWS
	cognitoClient  *CognitoClient
	selectedRegion string

	// Pools
	poolList     list.Model
	selectedPool *PoolItem

	// Users
	usersTable      table.Model
	users           []UserItem
	usersNextToken  string
	filterInput     textinput.Model
	filterMode      bool
	activeFilter    string

	// User detail
	selectedUser  *UserDetail
	actionCursor  int
	statusMessage string
	statusIsError bool

	// Confirm dialog
	pendingAction Action
	confirmMsg    string

	// Error
	err error
}

func NewApp() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = headerStyle

	fi := textinput.New()
	fi.Placeholder = "username prefix, email:x@y.com, or field:value"
	fi.CharLimit = 120

	pl := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 20)
	pl.Title = "Select AWS Profile"
	pl.SetShowStatusBar(false)
	pl.Styles.Title = titleStyle

	poolL := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 20)
	poolL.Title = "Select User Pool"
	poolL.SetShowStatusBar(false)
	poolL.Styles.Title = titleStyle

	columns := []table.Column{
		{Title: "Username", Width: 28},
		{Title: "Status", Width: 22},
		{Title: "On", Width: 3},
		{Title: "Email", Width: 32},
		{Title: "Created", Width: 19},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(20),
	)
	ts := table.DefaultStyles()
	ts.Header = ts.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		BorderBottom(true).
		Bold(true)
	ts.Selected = ts.Selected.
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Bold(true)
	t.SetStyles(ts)

	return Model{
		screen:      int(ScreenLoading),
		spinner:     s,
		loadingMsg:  "Loading AWS profiles…",
		profileList: pl,
		poolList:    poolL,
		usersTable:  t,
		filterInput: fi,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadProfilesCmd())
}

// userActions returns the ordered list of actions available for the selected user.
func (m Model) userActions() []string {
	if m.selectedUser == nil {
		return nil
	}
	var actions []string
	if m.selectedUser.Enabled {
		actions = append(actions, "Disable User")
	} else {
		actions = append(actions, "Enable User")
	}
	if m.selectedUser.Status == "UNCONFIRMED" {
		actions = append(actions, "Confirm Sign Up")
	}
	actions = append(actions, "Reset Password", "Sign Out All Devices", "Delete User")
	return actions
}

// refreshUsersTable rebuilds the table rows from m.users.
func refreshUsersTable(m Model) Model {
	rows := make([]table.Row, len(m.users))
	for i, u := range m.users {
		on := "✓"
		if !u.Enabled {
			on = "✗"
		}
		rows[i] = table.Row{
			u.Username,
			u.Status,
			on,
			u.Email,
			u.CreatedDate.Format("2006-01-02 15:04"),
		}
	}
	m.usersTable.SetRows(rows)
	return m
}
