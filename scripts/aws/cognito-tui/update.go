package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// ── Global handlers (fire on every screen) ────────────────────────────────
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.profileList.SetSize(msg.Width-4, msg.Height-8)
		m.poolList.SetSize(msg.Width-4, msg.Height-8)
		m.usersTable.SetHeight(msg.Height - 13)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	// ── Async results ─────────────────────────────────────────────────────────
	case profilesLoadedMsg:
		items := make([]list.Item, len(msg.profiles))
		for i, p := range msg.profiles {
			items[i] = profileItem(p)
		}
		m.profileList.SetItems(items)
		m.screen = int(ScreenProfiles)
		return m, nil

	case profilesErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil

	case clientCreatedMsg:
		m.cognitoClient = msg.client
		m.selectedRegion = msg.client.Region
		m.loadingMsg = "Loading user pools…"
		return m, loadPoolsCmd(m.cognitoClient)

	case clientErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil

	case poolsLoadedMsg:
		items := make([]list.Item, len(msg.pools))
		for i, p := range msg.pools {
			items[i] = poolListItem{p}
		}
		m.poolList.SetItems(items)
		m.poolList.Title = fmt.Sprintf("User Pools  ·  %s / %s", m.selectedProfile, m.selectedRegion)
		m.screen = int(ScreenPools)
		return m, nil

	case poolsErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil

	case usersLoadedMsg:
		if msg.append {
			m.users = append(m.users, msg.users...)
		} else {
			m.users = msg.users
		}
		m.usersNextToken = msg.nextToken
		m = refreshUsersTable(m)
		m.screen = int(ScreenUsers)
		return m, nil

	case usersErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil

	case userDetailLoadedMsg:
		m.selectedUser = msg.detail
		m.actionCursor = 0
		m.screen = int(ScreenUserDetail)
		return m, nil

	case userDetailErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil

	case actionSuccessMsg:
		m.statusMessage = msg.msg
		m.statusIsError = false
		m.pendingAction = ActionNone
		if msg.action == ActionDelete {
			// Go back to users list and reload.
			m.selectedUser = nil
			m.users = nil
			m.screen = int(ScreenLoading)
			m.loadingMsg = "Reloading users…"
			filter := buildFilter(m.activeFilter)
			return m, loadUsersCmd(m.cognitoClient, m.selectedPool.ID, filter, "", false)
		}
		// Reload user detail to reflect the change.
		m.screen = int(ScreenLoading)
		m.loadingMsg = "Refreshing user…"
		return m, loadUserDetailCmd(m.cognitoClient, m.selectedPool.ID, m.selectedUser.Username)

	case actionErrMsg:
		m.statusMessage = msg.err.Error()
		m.statusIsError = true
		m.pendingAction = ActionNone
		m.screen = int(ScreenUserDetail)
		return m, nil
	}

	// ── Screen-specific routing ───────────────────────────────────────────────
	switch Screen(m.screen) {
	case ScreenProfiles:
		return m.updateProfiles(msg)
	case ScreenPools:
		return m.updatePools(msg)
	case ScreenUsers:
		return m.updateUsers(msg)
	case ScreenUserDetail:
		return m.updateUserDetail(msg)
	case ScreenConfirm:
		return m.updateConfirm(msg)
	case ScreenError:
		return m.updateError(msg)
	}

	return m, nil
}

func (m Model) updateProfiles(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			if item, ok := m.profileList.SelectedItem().(profileItem); ok {
				m.selectedProfile = string(item)
				region := GetProfileRegion(m.selectedProfile)
				m.loadingMsg = fmt.Sprintf("Connecting to AWS (%s)…", m.selectedProfile)
				m.screen = int(ScreenLoading)
				return m, createClientCmd(m.selectedProfile, region)
			}
			return m, nil
		case "q":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.profileList, cmd = m.profileList.Update(msg)
	return m, cmd
}

func (m Model) updatePools(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			if item, ok := m.poolList.SelectedItem().(poolListItem); ok {
				pool := item.pool
				m.selectedPool = &pool
				m.activeFilter = ""
				m.users = nil
				m.loadingMsg = fmt.Sprintf("Loading users from %s…", pool.Name)
				m.screen = int(ScreenLoading)
				return m, loadUsersCmd(m.cognitoClient, pool.ID, "", "", false)
			}
			return m, nil
		case "esc", "backspace":
			m.screen = int(ScreenProfiles)
			return m, nil
		case "r":
			m.loadingMsg = "Refreshing pools…"
			m.screen = int(ScreenLoading)
			return m, loadPoolsCmd(m.cognitoClient)
		case "q":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.poolList, cmd = m.poolList.Update(msg)
	return m, cmd
}

func (m Model) updateUsers(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Filter input mode intercepts all keys.
		if m.filterMode {
			switch msg.String() {
			case "enter":
				m.activeFilter = m.filterInput.Value()
				m.filterMode = false
				m.filterInput.Blur()
				m.users = nil
				m.loadingMsg = "Filtering users…"
				m.screen = int(ScreenLoading)
				return m, loadUsersCmd(m.cognitoClient, m.selectedPool.ID, buildFilter(m.activeFilter), "", false)
			case "esc":
				m.filterMode = false
				m.filterInput.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "enter":
			row := m.usersTable.SelectedRow()
			if len(row) > 0 {
				username := row[0]
				m.loadingMsg = fmt.Sprintf("Loading %s…", username)
				m.screen = int(ScreenLoading)
				return m, loadUserDetailCmd(m.cognitoClient, m.selectedPool.ID, username)
			}
		case "esc", "backspace":
			m.screen = int(ScreenPools)
			return m, nil
		case "/":
			m.filterMode = true
			m.filterInput.Focus()
			return m, nil
		case "c":
			m.activeFilter = ""
			m.filterInput.SetValue("")
			m.users = nil
			m.loadingMsg = "Loading users…"
			m.screen = int(ScreenLoading)
			return m, loadUsersCmd(m.cognitoClient, m.selectedPool.ID, "", "", false)
		case "n":
			if m.usersNextToken != "" {
				m.loadingMsg = "Loading more users…"
				m.screen = int(ScreenLoading)
				return m, loadUsersCmd(m.cognitoClient, m.selectedPool.ID, buildFilter(m.activeFilter), m.usersNextToken, true)
			}
		case "r":
			m.users = nil
			m.loadingMsg = "Refreshing users…"
			m.screen = int(ScreenLoading)
			return m, loadUsersCmd(m.cognitoClient, m.selectedPool.ID, buildFilter(m.activeFilter), "", false)
		case "q":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.usersTable, cmd = m.usersTable.Update(msg)
	return m, cmd
}

func (m Model) updateUserDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	actions := m.userActions()
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			m.statusMessage = ""
			m.screen = int(ScreenUsers)
			return m, nil
		case "up", "k":
			if m.actionCursor > 0 {
				m.actionCursor--
			}
		case "down", "j":
			if m.actionCursor < len(actions)-1 {
				m.actionCursor++
			}
		case "enter":
			if len(actions) > 0 {
				return m.executeAction(actions[m.actionCursor])
			}
		case "r":
			m.statusMessage = ""
			m.loadingMsg = "Refreshing user…"
			m.screen = int(ScreenLoading)
			return m, loadUserDetailCmd(m.cognitoClient, m.selectedPool.ID, m.selectedUser.Username)
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) executeAction(action string) (Model, tea.Cmd) {
	m.statusMessage = "" // clear previous status before new action
	switch action {
	case "Enable User":
		m.pendingAction = ActionEnable
		m.confirmMsg = fmt.Sprintf("Enable user  %q ?", m.selectedUser.Username)
	case "Disable User":
		m.pendingAction = ActionDisable
		m.confirmMsg = fmt.Sprintf("Disable user  %q ?", m.selectedUser.Username)
	case "Delete User":
		m.pendingAction = ActionDelete
		m.confirmMsg = fmt.Sprintf("⚠  Permanently delete user  %q ?\nThis cannot be undone.", m.selectedUser.Username)
	case "Reset Password":
		m.pendingAction = ActionResetPassword
		m.confirmMsg = fmt.Sprintf("Reset password for  %q ?\nAWS will send the user a reset code.", m.selectedUser.Username)
	case "Confirm Sign Up":
		m.pendingAction = ActionConfirmUser
		m.confirmMsg = fmt.Sprintf("Confirm sign-up for  %q ?", m.selectedUser.Username)
	case "Sign Out All Devices":
		m.pendingAction = ActionSignOut
		m.confirmMsg = fmt.Sprintf("Sign out  %q  from all devices?", m.selectedUser.Username)
	default:
		return m, nil
	}
	m.screen = int(ScreenConfirm)
	return m, nil
}

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter":
			return m.runPendingAction()
		case "n", "esc":
			m.pendingAction = ActionNone
			m.screen = int(ScreenUserDetail)
			return m, nil
		}
	}
	return m, nil
}

func (m Model) runPendingAction() (Model, tea.Cmd) {
	poolID := m.selectedPool.ID
	username := m.selectedUser.Username
	m.loadingMsg = "Applying change…"
	m.screen = int(ScreenLoading)
	switch m.pendingAction {
	case ActionEnable:
		return m, enableUserCmd(m.cognitoClient, poolID, username)
	case ActionDisable:
		return m, disableUserCmd(m.cognitoClient, poolID, username)
	case ActionDelete:
		return m, deleteUserCmd(m.cognitoClient, poolID, username)
	case ActionResetPassword:
		return m, resetPasswordCmd(m.cognitoClient, poolID, username)
	case ActionConfirmUser:
		return m, confirmUserCmd(m.cognitoClient, poolID, username)
	case ActionSignOut:
		return m, signOutUserCmd(m.cognitoClient, poolID, username)
	}
	return m, nil
}

func (m Model) updateError(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "esc", "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}
