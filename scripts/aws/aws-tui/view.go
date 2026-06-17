package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	switch Screen(m.screen) {
	case ScreenProfiles:
		return m.viewProfiles()
	case ScreenLoading:
		return m.viewLoading()
	case ScreenServiceSelect:
		return m.viewServiceSelect()
	case ScreenPools:
		return m.viewPools()
	case ScreenUsers:
		return m.viewUsers()
	case ScreenUserDetail:
		return m.viewUserDetail()
	case ScreenConfirm:
		return m.viewConfirm()
	case ScreenDynamoTables:
		return m.viewDynamoTables()
	case ScreenDynamoItems:
		return m.viewDynamoItems()
	case ScreenDynamoDetail:
		return m.viewDynamoDetail()
	case ScreenLambdaFunctions:
		return m.viewLambdaFunctions()
	case ScreenLambdaLogStreams:
		return m.viewLambdaLogStreams()
	case ScreenLambdaLogs:
		return m.viewLambdaLogs()
	case ScreenAPIGateways:
		return m.viewAPIGateways()
	case ScreenAPIDetail:
		return m.viewAPIDetail()
	case ScreenError:
		return m.viewError()
	}
	return ""
}

func (m Model) topBar(subtitle string) string {
	left := appNameStyle.Render("☁  AWS TUI")
	right := dimStyle.Render(subtitle)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	bar := left + strings.Repeat(" ", gap) + right
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#1a1a2e")).
		Width(m.width).
		Padding(0, 1).
		Render(bar)
}

func footer(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, keyStyle.Render(pairs[i])+" "+dimStyle.Render(pairs[i+1]))
	}
	return "  " + strings.Join(parts, dimStyle.Render("  ·  "))
}

func (m Model) viewLoading() string {
	bar := m.topBar("")
	body := fmt.Sprintf("\n\n  %s  %s\n", m.spinner.View(), infoStyle.Render(m.loadingMsg))
	return lipgloss.JoinVertical(lipgloss.Left, bar, body)
}

func (m Model) viewProfiles() string {
	bar := m.topBar("profile selection")
	f := footer("↑↓", "navigate", "enter", "select", "/", "filter", "q", "quit")
	content := lipgloss.NewStyle().PaddingLeft(2).Render(m.profileList.View())
	return lipgloss.JoinVertical(lipgloss.Left, bar, content, f)
}

func (m Model) viewServiceSelect() string {
	bar := m.topBar(fmt.Sprintf("%s / %s", m.selectedProfile, m.selectedRegion))
	f := footer("↑↓", "navigate", "enter", "select", "esc", "back", "q", "quit")
	content := lipgloss.NewStyle().PaddingLeft(2).Render(m.serviceList.View())
	return lipgloss.JoinVertical(lipgloss.Left, bar, content, f)
}

func (m Model) viewPools() string {
	bar := m.topBar(fmt.Sprintf("Cognito  ·  %s / %s", m.selectedProfile, m.selectedRegion))
	f := footer("↑↓", "navigate", "enter", "select", "r", "refresh", "esc", "back", "q", "quit")
	content := lipgloss.NewStyle().PaddingLeft(2).Render(m.poolList.View())
	return lipgloss.JoinVertical(lipgloss.Left, bar, content, f)
}

func (m Model) viewUsers() string {
	pool := ""
	if m.selectedPool != nil {
		pool = m.selectedPool.Name
	}
	bar := m.topBar(fmt.Sprintf("Cognito  ·  %s / %s / %s", m.selectedProfile, m.selectedRegion, pool))

	var filterBar string
	if m.filterMode {
		filterBar = "\n  " + keyStyle.Render("Filter › ") + m.filterInput.View() + "\n"
	} else if m.activeFilter != "" {
		filterBar = "\n  " + keyStyle.Render("Filter › ") +
			infoStyle.Render(m.activeFilter) +
			dimStyle.Render("  (c to clear)") + "\n"
	} else {
		filterBar = "\n"
	}

	countMsg := fmt.Sprintf("%d users", len(m.users))
	if m.usersNextToken != "" {
		countMsg += "  (n → load more)"
	}
	counts := "  " + infoStyle.Render(countMsg) + "\n"

	tbl := lipgloss.NewStyle().PaddingLeft(2).Render(m.usersTable.View())
	f := footer("/", "filter", "c", "clear", "n", "next page", "r", "refresh", "enter", "details", "esc", "back", "q", "quit")
	return lipgloss.JoinVertical(lipgloss.Left, bar, filterBar, counts, tbl, "\n"+f)
}

func (m Model) viewUserDetail() string {
	if m.selectedUser == nil {
		return "no user selected"
	}
	u := m.selectedUser
	bar := m.topBar(fmt.Sprintf("Cognito  ·  user: %s", u.Username))

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + headerStyle.Render("User") + "\n")
	b.WriteString(row("Username", u.Username))
	b.WriteString(row("Status", statusStyle(u.Status).Render(u.Status)))
	if u.Enabled {
		b.WriteString(row("Account", enabledStyle(true).Render("Enabled")))
	} else {
		b.WriteString(row("Account", enabledStyle(false).Render("Disabled")))
	}
	b.WriteString(row("Created", u.CreatedDate.Format("2006-01-02 15:04:05 UTC")))
	if len(u.Groups) > 0 {
		b.WriteString(row("Groups", strings.Join(u.Groups, ", ")))
	}
	if len(u.MFAOptions) > 0 {
		b.WriteString(row("MFA", strings.Join(u.MFAOptions, ", ")))
	}

	if len(u.Attributes) > 0 {
		b.WriteString("\n  " + headerStyle.Render("Attributes") + "\n")
		keys := make([]string, 0, len(u.Attributes))
		for k := range u.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(row(k, u.Attributes[k]))
		}
	}

	b.WriteString("\n  " + headerStyle.Render("Actions") + "\n")
	actions := m.userActions()
	for i, a := range actions {
		cursor := "  "
		var label lipgloss.Style
		if i == m.actionCursor {
			cursor = "▶ "
			label = selectedRowStyle
		} else {
			switch a {
			case "Delete User":
				label = lipgloss.NewStyle().Foreground(lipgloss.Color("#E84855"))
			case "Disable User":
				label = lipgloss.NewStyle().Foreground(lipgloss.Color("#F5A623"))
			case "Enable User":
				label = lipgloss.NewStyle().Foreground(lipgloss.Color("#43BF6D"))
			default:
				label = lipgloss.NewStyle()
			}
		}
		b.WriteString("  " + cursor + label.Render(a) + "\n")
	}

	if m.statusMessage != "" {
		b.WriteString("\n  ")
		if m.statusIsError {
			b.WriteString(errorStyle.Render("✗ " + m.statusMessage))
		} else {
			b.WriteString(successStyle.Render("✓ " + m.statusMessage))
		}
		b.WriteString("\n")
	}

	f := footer("↑↓/jk", "select action", "enter", "execute", "r", "refresh", "esc", "back", "q", "quit")
	return lipgloss.JoinVertical(lipgloss.Left, bar, b.String(), f)
}

func (m Model) viewDynamoTables() string {
	bar := m.topBar(fmt.Sprintf("DynamoDB  ·  %s / %s", m.selectedProfile, m.selectedRegion))
	f := footer("↑↓", "navigate", "enter", "open", "r", "refresh", "esc", "back", "q", "quit")
	content := lipgloss.NewStyle().PaddingLeft(2).Render(m.dynamoTableList.View())
	return lipgloss.JoinVertical(lipgloss.Left, bar, content, f)
}

func (m Model) viewDynamoItems() string {
	tableName := ""
	if m.selectedDynamoTable != nil {
		tableName = m.selectedDynamoTable.Name
	}
	bar := m.topBar(fmt.Sprintf("DynamoDB  ·  %s / %s / %s", m.selectedProfile, m.selectedRegion, tableName))

	var filterBar string
	if m.dynamoFilterMode {
		filterBar = "\n  " + keyStyle.Render("Filter › ") + m.dynamoFilterInput.View() + "\n"
	} else if m.dynamoActiveFilter != "" {
		filterBar = "\n  " + keyStyle.Render("Filter › ") +
			infoStyle.Render(m.dynamoActiveFilter) +
			dimStyle.Render("  (c to clear)") + "\n"
	} else {
		filterBar = "\n"
	}

	countMsg := fmt.Sprintf("%d items", len(m.dynamoItems))
	if m.dynamoNextKey != nil {
		countMsg += "  (n → load more)"
	}
	counts := "  " + infoStyle.Render(countMsg) + "\n"

	tbl := lipgloss.NewStyle().PaddingLeft(2).Render(m.dynamoItemsTable.View())
	f := footer("/", "filter", "c", "clear", "n", "next page", "r", "refresh", "enter", "details", "esc", "back", "q", "quit")
	return lipgloss.JoinVertical(lipgloss.Left, bar, filterBar, counts, tbl, "\n"+f)
}

func (m Model) viewDynamoDetail() string {
	if m.selectedDynamoItem == nil {
		return "no item selected"
	}
	tableName := ""
	if m.selectedDynamoTable != nil {
		tableName = m.selectedDynamoTable.Name
	}
	bar := m.topBar(fmt.Sprintf("DynamoDB  ·  %s / item", tableName))

	var b strings.Builder
	b.WriteString("\n  " + headerStyle.Render("Item Attributes") + "\n\n")

	for i, a := range m.selectedDynamoItem.Attrs {
		cursor := "  "
		keyS := dimStyle.Render(a.Key + ":")
		valS := a.Value
		if i == m.dynamoDetailCursor {
			cursor = "▶ "
			keyS = selectedRowStyle.Render(a.Key + ":")
			valS = lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Render(a.Value)
		}
		b.WriteString(fmt.Sprintf("  %s%-30s %s\n", cursor, keyS, valS))
	}

	if m.statusMessage != "" {
		b.WriteString("\n  ")
		if m.statusIsError {
			b.WriteString(errorStyle.Render("✗ " + m.statusMessage))
		} else {
			b.WriteString(successStyle.Render("✓ " + m.statusMessage))
		}
		b.WriteString("\n")
	}

	f := footer("↑↓/jk", "scroll", "d", "delete item", "esc", "back", "q", "quit")
	return lipgloss.JoinVertical(lipgloss.Left, bar, b.String(), f)
}

func (m Model) viewConfirm() string {
	bar := m.topBar("confirm")
	style := warningStyle
	if m.pendingAction == ActionDelete || m.pendingAction == ActionDynamoDeleteItem {
		style = errorStyle
	}
	body := fmt.Sprintf("\n\n  %s\n\n  %s  %s\n  %s  %s\n\n",
		style.Render(m.confirmMsg),
		keyStyle.Render("y / enter"), dimStyle.Render("confirm"),
		keyStyle.Render("n / esc  "), dimStyle.Render("cancel"),
	)
	return lipgloss.JoinVertical(lipgloss.Left, bar, body)
}

func (m Model) viewError() string {
	bar := m.topBar("error")
	body := fmt.Sprintf("\n\n  %s\n\n  %s\n\n  %s\n",
		errorStyle.Render("An error occurred:"),
		errorStyle.Render(m.err.Error()),
		dimStyle.Render("press enter or q to exit"),
	)
	return lipgloss.JoinVertical(lipgloss.Left, bar, body)
}

func (m Model) viewLambdaFunctions() string {
	bar := m.topBar(fmt.Sprintf("Lambda  ·  %s / %s", m.selectedProfile, m.selectedRegion))
	f := footer("↑↓", "navigate", "/", "search", "enter", "log streams", "r", "refresh", "esc", "back", "q", "quit")
	content := lipgloss.NewStyle().PaddingLeft(2).Render(m.lambdaList.View())
	return lipgloss.JoinVertical(lipgloss.Left, bar, content, f)
}

func (m Model) viewLambdaLogStreams() string {
	fnName := ""
	if m.selectedLambda != nil {
		fnName = m.selectedLambda.Name
	}
	bar := m.topBar(fmt.Sprintf("Lambda  ·  %s / %s / %s", m.selectedProfile, m.selectedRegion, fnName))

	var detail strings.Builder
	if m.selectedLambda != nil {
		fn := m.selectedLambda
		detail.WriteString("\n")
		detail.WriteString(row("Runtime", fn.Runtime))
		detail.WriteString(row("Handler", fn.Handler))
		detail.WriteString(row("Memory", fmt.Sprintf("%d MB", fn.MemorySize)))
		detail.WriteString(row("Timeout", fmt.Sprintf("%d s", fn.Timeout)))
		if fn.Description != "" {
			detail.WriteString(row("Description", fn.Description))
		}
		detail.WriteString(row("State", fn.State))
		detail.WriteString(row("Log Group", fn.LogGroup))
		detail.WriteString("\n")
	}

	var statusBar string
	if m.lambdaStatusMsg != "" {
		statusBar = "\n  " + warningStyle.Render("⚠  "+m.lambdaStatusMsg) + "\n"
	}

	content := lipgloss.NewStyle().PaddingLeft(2).Render(m.lambdaStreamList.View())
	f := footer("↑↓", "navigate", "enter", "view logs", "r", "refresh", "esc", "back", "q", "quit")
	return lipgloss.JoinVertical(lipgloss.Left, bar, detail.String(), statusBar, content, f)
}

func (m Model) viewLambdaLogs() string {
	streamName := ""
	if m.selectedLogStream != nil {
		streamName = m.selectedLogStream.Name
	}
	bar := m.topBar(fmt.Sprintf("Lambda  ·  logs  ·  %s", streamName))

	countMsg := fmt.Sprintf("%d events", len(m.lambdaLogEvents))
	counts := "  " + infoStyle.Render(countMsg) + "\n"

	vp := lipgloss.NewStyle().PaddingLeft(2).Render(m.lambdaLogsViewport.View())
	f := footer("↑↓/PgUp/PgDn", "scroll", "g/G", "top/bottom", "r", "refresh", "esc", "back", "q", "quit")
	return lipgloss.JoinVertical(lipgloss.Left, bar, counts, vp, "\n"+f)
}

func row(label, value string) string {
	return fmt.Sprintf("  %-28s %s\n", dimStyle.Render(label+":"), value)
}

func (m Model) viewAPIGateways() string {
	bar := m.topBar(fmt.Sprintf("API Gateway  ·  %s / %s", m.selectedProfile, m.selectedRegion))
	f := footer("↑↓", "navigate", "/", "search", "enter", "details", "r", "refresh", "esc", "back", "q", "quit")
	content := lipgloss.NewStyle().PaddingLeft(2).Render(m.apigwList.View())
	return lipgloss.JoinVertical(lipgloss.Left, bar, content, f)
}

func (m Model) viewAPIDetail() string {
	apiType := ""
	if m.selectedAPI != nil {
		apiType = m.selectedAPI.Type
	}
	bar := m.topBar(fmt.Sprintf("API Gateway  ·  %s / %s  [%s]", m.selectedProfile, m.selectedRegion, apiType))

	// Compact stages header
	var stagesHeader strings.Builder
	if m.apigwDetail != nil && len(m.apigwDetail.Stages) > 0 {
		stagesHeader.WriteString("  " + dimStyle.Render("Stages:") + "\n")
		for _, s := range m.apigwDetail.Stages {
			stagesHeader.WriteString(fmt.Sprintf("  %s  %s\n",
				keyStyle.Render(s.Name), dimStyle.Render(s.InvokeURL)))
		}
	} else if m.apigwDetail != nil {
		stagesHeader.WriteString("  " + dimStyle.Render("(no stages)") + "\n")
	} else {
		stagesHeader.WriteString("  " + dimStyle.Render("Loading…") + "\n")
	}
	stagesHeader.WriteString("\n")

	content := lipgloss.NewStyle().PaddingLeft(2).Render(m.apigwRouteList.View())
	f := footer("↑↓", "navigate", "/", "search", "r", "refresh", "esc", "back", "q", "quit")
	return lipgloss.JoinVertical(lipgloss.Left, bar, stagesHeader.String(), content, "\n"+f)
}
