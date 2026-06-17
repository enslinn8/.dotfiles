package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen identifies the active TUI screen.
type Screen int

const (
	ScreenLoading       Screen = iota
	ScreenProfiles
	ScreenServiceSelect // choose Cognito or DynamoDB
	ScreenPools         // Cognito: user pool list
	ScreenUsers         // Cognito: users table
	ScreenUserDetail    // Cognito: single-user detail + actions
	ScreenConfirm       // y/n confirmation dialog (shared)
	ScreenError         // fatal error
	ScreenDynamoTables      // DynamoDB: table list
	ScreenDynamoItems       // DynamoDB: scan results
	ScreenDynamoDetail      // DynamoDB: single item detail
	ScreenLambdaFunctions   // Lambda: function list
	ScreenLambdaLogStreams   // Lambda: CloudWatch log stream list
	ScreenLambdaLogs        // Lambda: CloudWatch log event viewer
	ScreenAPIGateways       // API Gateway: API list
	ScreenAPIDetail         // API Gateway: stages + routes detail
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
	ActionDynamoDeleteItem
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

type serviceItem string

func (s serviceItem) FilterValue() string { return string(s) }
func (s serviceItem) Title() string       { return string(s) }
func (s serviceItem) Description() string {
	switch string(s) {
	case "Cognito":
		return "Manage user pools and users"
	case "DynamoDB":
		return "Browse and manage DynamoDB tables"
	case "Lambda":
		return "Browse functions and view CloudWatch logs"
	case "API Gateway":
		return "Browse REST, HTTP and WebSocket APIs"
	}
	return ""
}

type dynamoTableItem struct{ table DynamoTable }

func (d dynamoTableItem) FilterValue() string { return d.table.Name }
func (d dynamoTableItem) Title() string       { return d.table.Name }
func (d dynamoTableItem) Description() string {
	if d.table.SKName != "" {
		return "PK: " + d.table.PKName + " (" + d.table.PKType + ")  SK: " + d.table.SKName + " (" + d.table.SKType + ")  status: " + d.table.Status
	}
	return "PK: " + d.table.PKName + " (" + d.table.PKType + ")  status: " + d.table.Status
}

type lambdaFunctionItem struct{ fn LambdaFunction }

func (l lambdaFunctionItem) FilterValue() string { return l.fn.Name }
func (l lambdaFunctionItem) Title() string       { return l.fn.Name }
func (l lambdaFunctionItem) Description() string {
	desc := l.fn.Runtime
	if l.fn.Description != "" {
		desc += "  ·  " + l.fn.Description
	}
	if l.fn.State != "" && l.fn.State != "Active" {
		desc += "  ·  state: " + l.fn.State
	}
	return desc
}

type lambdaLogStreamItem struct{ stream LambdaLogStream }

func (l lambdaLogStreamItem) FilterValue() string { return l.stream.Name }
func (l lambdaLogStreamItem) Title() string       { return l.stream.Name }
func (l lambdaLogStreamItem) Description() string {
	if !l.stream.LastEventTime.IsZero() {
		return "last event: " + l.stream.LastEventTime.Format("2006-01-02 15:04:05 UTC")
	}
	return "created: " + l.stream.CreationTime.Format("2006-01-02 15:04:05 UTC")
}

type apigwAPIItem struct{ api APIGatewayAPI }

func (a apigwAPIItem) FilterValue() string { return a.api.Name + " " + a.api.ID }
func (a apigwAPIItem) Title() string       { return a.api.Name }
func (a apigwAPIItem) Description() string {
	desc := a.api.Type + "  ·  " + a.api.ID
	if a.api.Description != "" {
		desc += "  ·  " + a.api.Description
	}
	return desc
}

type apigwRouteItem struct{ route APIGatewayRoute }

func (r apigwRouteItem) FilterValue() string { return r.route.Key + " " + r.route.Target }
func (r apigwRouteItem) Title() string       { return r.route.Key }
func (r apigwRouteItem) Description() string {
	if r.route.Target != "" {
		return "→ " + r.route.Target
	}
	return ""
}

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	screen int
	width  int
	height int

	// Loading
	spinner    spinner.Model
	loadingMsg string

	// Profile
	profileList     list.Model
	selectedProfile string

	// Service selector
	serviceList list.Model

	// AWS shared
	selectedRegion string

	// Cognito
	cognitoClient  *CognitoClient
	poolList        list.Model
	selectedPool    *PoolItem
	usersTable      table.Model
	users           []UserItem
	usersNextToken  string
	filterInput     textinput.Model
	filterMode      bool
	activeFilter    string
	selectedUser    *UserDetail
	actionCursor    int
	statusMessage   string
	statusIsError   bool

	// Confirm (shared)
	pendingAction Action
	confirmMsg    string

	// DynamoDB
	dynamoClient        *DynamoClient
	dynamoTableList     list.Model
	dynamoTables        []DynamoTable
	selectedDynamoTable *DynamoTable
	dynamoItemsTable    table.Model
	dynamoItems         []DynamoItem
	dynamoNextKey       map[string]types.AttributeValue
	dynamoFilterInput   textinput.Model
	dynamoFilterMode    bool
	dynamoActiveFilter  string
	selectedDynamoItem  *DynamoItem
	dynamoDetailCursor  int

	// Lambda
	lambdaClient       *LambdaClient
	lambdaList         list.Model
	lambdaFunctions    []LambdaFunction
	selectedLambda     *LambdaFunction
	lambdaStreamList   list.Model
	lambdaLogStreams    []LambdaLogStream
	selectedLogStream  *LambdaLogStream
	lambdaLogsViewport viewport.Model
	lambdaLogEvents    []LambdaLogEvent
	lambdaStatusMsg    string

	// API Gateway
	apigwClient     *APIGatewayClient
	apigwList       list.Model
	apigwAPIs       []APIGatewayAPI
	selectedAPI     *APIGatewayAPI
	apigwDetail     *APIGatewayDetail
	apigwRouteList  list.Model

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

	dfi := textinput.New()
	dfi.Placeholder = "DynamoDB filter expression, e.g. attribute_exists(pk)"
	dfi.CharLimit = 240

	pl := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 20)
	pl.Title = "Select AWS Profile"
	pl.SetShowStatusBar(false)
	pl.Styles.Title = titleStyle

	sl := list.New([]list.Item{
		serviceItem("Cognito"),
		serviceItem("DynamoDB"),
		serviceItem("Lambda"),
		serviceItem("API Gateway"),
	}, list.NewDefaultDelegate(), 80, 20)
	sl.Title = "Select Service"
	sl.SetShowStatusBar(false)
	sl.Styles.Title = titleStyle

	poolL := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 20)
	poolL.Title = "Select User Pool"
	poolL.SetShowStatusBar(false)
	poolL.Styles.Title = titleStyle

	dtl := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 20)
	dtl.Title = "DynamoDB Tables"
	dtl.SetShowStatusBar(false)
	dtl.Styles.Title = titleStyle

	ll := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 20)
	ll.Title = "Lambda Functions"
	ll.SetShowStatusBar(false)
	ll.Styles.Title = titleStyle

	lsl := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 20)
	lsl.Title = "Log Streams"
	lsl.SetShowStatusBar(false)
	lsl.Styles.Title = titleStyle

	agl := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 20)
	agl.Title = "API Gateway"
	agl.SetShowStatusBar(false)
	agl.Styles.Title = titleStyle

	arl := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 20)
	arl.Title = "Routes"
	arl.SetShowStatusBar(false)
	arl.Styles.Title = titleStyle

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

	dynoCols := []table.Column{
		{Title: "#", Width: 4},
		{Title: "Preview (first 3 attrs)", Width: 80},
	}
	dt := table.New(
		table.WithColumns(dynoCols),
		table.WithFocused(true),
		table.WithHeight(20),
	)
	dt.SetStyles(ts)

	return Model{
		screen:             int(ScreenLoading),
		spinner:            s,
		loadingMsg:         "Loading AWS profiles…",
		profileList:        pl,
		serviceList:        sl,
		poolList:           poolL,
		usersTable:         t,
		filterInput:        fi,
		dynamoTableList:    dtl,
		dynamoItemsTable:   dt,
		dynamoFilterInput:  dfi,
		lambdaList:         ll,
		lambdaStreamList:   lsl,
		lambdaLogsViewport: viewport.New(80, 20),
		apigwList:          agl,
		apigwRouteList:     arl,
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

// refreshDynamoItemsTable rebuilds dynamo items table rows.
func refreshDynamoItemsTable(m Model) Model {
	rows := make([]table.Row, len(m.dynamoItems))
	for i, item := range m.dynamoItems {
		preview := ""
		for j, a := range item.Attrs {
			if j >= 3 {
				preview += "…"
				break
			}
			if j > 0 {
				preview += "  "
			}
			preview += a.Key + "=" + a.Value
		}
		rows[i] = table.Row{fmt.Sprintf("%d", i+1), preview}
	}
	m.dynamoItemsTable.SetRows(rows)
	return m
}
