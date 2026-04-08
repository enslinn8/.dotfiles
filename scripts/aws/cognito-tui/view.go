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
	case ScreenPools:
		return m.viewPools()
	case ScreenUsers:
		return m.viewUsers()
	case ScreenUserDetail:
		return m.viewUserDetail()
	case ScreenConfirm:
		return m.viewConfirm()
	case ScreenError:
		return m.viewError()
	}
	return ""
}

// ── Layout helpers ────────────────────────────────────────────────────────────

func (m Model) topBar(subtitle string) string {
	left := appNameStyle.Render("🔐 Cognito TUI")
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

// ── Screens ───────────────────────────────────────────────────────────────────

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

func (m Model) viewPools() string {
	bar := m.topBar(fmt.Sprintf("%s / %s", m.selectedProfile, m.selectedRegion))
	f := footer("↑↓", "navigate", "enter", "select", "r", "refresh", "esc", "back", "q", "quit")
	content := lipgloss.NewStyle().PaddingLeft(2).Render(m.poolList.View())
	return lipgloss.JoinVertical(lipgloss.Left, bar, content, f)
}

func (m Model) viewUsers() string {
	pool := ""
	if m.selectedPool != nil {
		pool = m.selectedPool.Name
	}
	bar := m.topBar(fmt.Sprintf("%s / %s / %s", m.selectedProfile, m.selectedRegion, pool))

	// Filter bar
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

	// Count row
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
	bar := m.topBar(fmt.Sprintf("user: %s", u.Username))

	var b strings.Builder
	b.WriteString("\n")

	// ── Basic info ─────────────────────────────────────────────────────────
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

	// ── Attributes ─────────────────────────────────────────────────────────
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

	// ── Actions ────────────────────────────────────────────────────────────
	b.WriteString("\n  " + headerStyle.Render("Actions") + "\n")
	actions := m.userActions()
	for i, a := range actions {
		cursor := "  "
		label := lipgloss.NewStyle()
		if i == m.actionCursor {
			cursor = "▶ "
			label = selectedRowStyle
		}
		switch a {
		case "Delete User":
			label = label.Copy().Foreground(lipgloss.Color("#E84855"))
		case "Disable User":
			label = label.Copy().Foreground(lipgloss.Color("#F5A623"))
		case "Enable User":
			label = label.Copy().Foreground(lipgloss.Color("#43BF6D"))
		}
		if i == m.actionCursor {
			label = selectedRowStyle
		}
		b.WriteString("  " + cursor + label.Render(a) + "\n")
	}

	// ── Status message ─────────────────────────────────────────────────────
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

func (m Model) viewConfirm() string {
	bar := m.topBar("confirm")
	style := warningStyle
	if m.pendingAction == ActionDelete {
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

// row renders a label/value pair with consistent alignment.
func row(label, value string) string {
	return fmt.Sprintf("  %-28s %s\n", dimStyle.Render(label+":"), value)
}
