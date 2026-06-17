package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.profileList.SetSize(msg.Width-4, msg.Height-8)
		m.serviceList.SetSize(msg.Width-4, msg.Height-8)
		m.poolList.SetSize(msg.Width-4, msg.Height-8)
		m.dynamoTableList.SetSize(msg.Width-4, msg.Height-8)
		m.lambdaList.SetSize(msg.Width-4, msg.Height-8)
		m.lambdaStreamList.SetSize(msg.Width-4, msg.Height-8)
		m.apigwList.SetSize(msg.Width-4, msg.Height-8)
		m.apigwRouteList.SetSize(msg.Width-4, msg.Height-14)
		m.usersTable.SetHeight(msg.Height - 13)
		m.dynamoItemsTable.SetHeight(msg.Height - 13)
		m.lambdaLogsViewport.Width = msg.Width - 4
		m.lambdaLogsViewport.Height = msg.Height - 8
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
		m.screen = int(ScreenServiceSelect)
		return m, nil

	case clientErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil

	case dynamoClientReadyMsg:
		m.dynamoClient = msg.client
		m.loadingMsg = "Loading DynamoDB tables…"
		return m, loadDynamoTablesCmd(m.dynamoClient)

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
			m.selectedUser = nil
			m.users = nil
			m.screen = int(ScreenLoading)
			m.loadingMsg = "Reloading users…"
			filter := buildFilter(m.activeFilter)
			return m, loadUsersCmd(m.cognitoClient, m.selectedPool.ID, filter, "", false)
		}
		m.screen = int(ScreenLoading)
		m.loadingMsg = "Refreshing user…"
		return m, loadUserDetailCmd(m.cognitoClient, m.selectedPool.ID, m.selectedUser.Username)

	case actionErrMsg:
		m.statusMessage = msg.err.Error()
		m.statusIsError = true
		m.pendingAction = ActionNone
		m.screen = int(ScreenUserDetail)
		return m, nil

	// ── DynamoDB async ────────────────────────────────────────────────────────
	case dynamoTablesLoadedMsg:
		m.dynamoTables = msg.tables
		items := make([]list.Item, len(msg.tables))
		for i, t := range msg.tables {
			items[i] = dynamoTableItem{t}
		}
		m.dynamoTableList.SetItems(items)
		m.dynamoTableList.Title = fmt.Sprintf("DynamoDB Tables  ·  %s / %s", m.selectedProfile, m.selectedRegion)
		m.screen = int(ScreenDynamoTables)
		return m, nil

	case dynamoTablesErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil

	case dynamoItemsLoadedMsg:
		if msg.appendMode {
			m.dynamoItems = append(m.dynamoItems, msg.items...)
		} else {
			m.dynamoItems = msg.items
		}
		m.dynamoNextKey = msg.nextKey
		m = refreshDynamoItemsTable(m)
		m.screen = int(ScreenDynamoItems)
		return m, nil

	case dynamoItemsErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil

	case dynamoDeleteSuccessMsg:
		m.statusMessage = "Item deleted"
		m.statusIsError = false
		m.pendingAction = ActionNone
		m.selectedDynamoItem = nil
		m.dynamoItems = nil
		m.screen = int(ScreenLoading)
		m.loadingMsg = "Reloading items…"
		return m, scanDynamoItemsCmd(m.dynamoClient, m.selectedDynamoTable.Name, m.dynamoActiveFilter, nil, false)

	case dynamoDeleteErrMsg:
		m.statusMessage = msg.err.Error()
		m.statusIsError = true
		m.pendingAction = ActionNone
		m.screen = int(ScreenDynamoDetail)
		return m, nil

	// ── Lambda async ──────────────────────────────────────────────────────────
	case lambdaClientReadyMsg:
		m.lambdaClient = msg.client
		m.loadingMsg = "Loading Lambda functions…"
		return m, loadLambdaFunctionsCmd(m.lambdaClient)

	case lambdaClientErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil

	case lambdaFunctionsLoadedMsg:
		m.lambdaFunctions = msg.fns
		items := make([]list.Item, len(msg.fns))
		for i, f := range msg.fns {
			items[i] = lambdaFunctionItem{f}
		}
		m.lambdaList.SetItems(items)
		m.lambdaList.Title = fmt.Sprintf("Lambda Functions  ·  %s / %s", m.selectedProfile, m.selectedRegion)
		m.screen = int(ScreenLambdaFunctions)
		return m, nil

	case lambdaFunctionsErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil

	case lambdaLogStreamsLoadedMsg:
		m.lambdaLogStreams = msg.streams
		items := make([]list.Item, len(msg.streams))
		for i, s := range msg.streams {
			items[i] = lambdaLogStreamItem{s}
		}
		m.lambdaStreamList.SetItems(items)
		if m.selectedLambda != nil {
			m.lambdaStreamList.Title = fmt.Sprintf("Log Streams  ·  %s", m.selectedLambda.Name)
		}
		m.lambdaStatusMsg = ""
		m.screen = int(ScreenLambdaLogStreams)
		return m, nil

	case lambdaLogStreamsErrMsg:
		// Show empty log streams screen with error message (log group may not exist yet).
		m.lambdaLogStreams = nil
		m.lambdaStreamList.SetItems(nil)
		if m.selectedLambda != nil {
			m.lambdaStreamList.Title = fmt.Sprintf("Log Streams  ·  %s", m.selectedLambda.Name)
		}
		m.lambdaStatusMsg = msg.err.Error()
		m.screen = int(ScreenLambdaLogStreams)
		return m, nil

	case lambdaLogEventsLoadedMsg:
		m.lambdaLogEvents = msg.events
		m.lambdaLogsViewport.SetContent(formatLogEvents(msg.events, m.width))
		m.lambdaLogsViewport.GotoBottom()
		m.screen = int(ScreenLambdaLogs)
		return m, nil

	case lambdaLogEventsErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil

	// ── API Gateway async ─────────────────────────────────────────────────────
	case apigwClientReadyMsg:
		m.apigwClient = msg.client
		m.loadingMsg = "Loading APIs…"
		return m, loadAPIGatewayAPIsCmd(m.apigwClient)

	case apigwClientErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil

	case apigwAPIsLoadedMsg:
		m.apigwAPIs = msg.apis
		items := make([]list.Item, len(msg.apis))
		for i, a := range msg.apis {
			items[i] = apigwAPIItem{a}
		}
		m.apigwList.SetItems(items)
		m.apigwList.Title = fmt.Sprintf("API Gateway  ·  %s / %s", m.selectedProfile, m.selectedRegion)
		m.screen = int(ScreenAPIGateways)
		return m, nil

	case apigwAPIsErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil

	case apigwDetailLoadedMsg:
		m.apigwDetail = msg.detail
		routeItems := make([]list.Item, len(msg.detail.Routes))
		for i, r := range msg.detail.Routes {
			routeItems[i] = apigwRouteItem{r}
		}
		m.apigwRouteList.SetItems(routeItems)
		m.apigwRouteList.Title = fmt.Sprintf("Routes  ·  %s", msg.detail.API.Name)
		m.apigwRouteList.ResetFilter()
		m.screen = int(ScreenAPIDetail)
		return m, nil

	case apigwDetailErrMsg:
		m.err = msg.err
		m.screen = int(ScreenError)
		return m, nil
	}

	switch Screen(m.screen) {
	case ScreenProfiles:
		return m.updateProfiles(msg)
	case ScreenServiceSelect:
		return m.updateServiceSelect(msg)
	case ScreenPools:
		return m.updatePools(msg)
	case ScreenUsers:
		return m.updateUsers(msg)
	case ScreenUserDetail:
		return m.updateUserDetail(msg)
	case ScreenConfirm:
		return m.updateConfirm(msg)
	case ScreenDynamoTables:
		return m.updateDynamoTables(msg)
	case ScreenDynamoItems:
		return m.updateDynamoItems(msg)
	case ScreenDynamoDetail:
		return m.updateDynamoDetail(msg)
	case ScreenLambdaFunctions:
		return m.updateLambdaFunctions(msg)
	case ScreenLambdaLogStreams:
		return m.updateLambdaLogStreams(msg)
	case ScreenLambdaLogs:
		return m.updateLambdaLogs(msg)
	case ScreenAPIGateways:
		return m.updateAPIGateways(msg)
	case ScreenAPIDetail:
		return m.updateAPIDetail(msg)
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

func (m Model) updateServiceSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.serviceList.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.serviceList, cmd = m.serviceList.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "enter", " ":
			item, ok := m.serviceList.SelectedItem().(serviceItem)
			if !ok {
				return m, nil
			}
			switch string(item) {
			case "Cognito":
				m.loadingMsg = "Loading user pools…"
				m.screen = int(ScreenLoading)
				return m, loadPoolsCmd(m.cognitoClient)
			case "DynamoDB":
				m.loadingMsg = "Connecting to DynamoDB…"
				m.screen = int(ScreenLoading)
				return m, createDynamoClientCmd(m.selectedProfile, m.selectedRegion)
			case "Lambda":
				m.loadingMsg = "Connecting to Lambda…"
				m.screen = int(ScreenLoading)
				return m, createLambdaClientCmd(m.selectedProfile, m.selectedRegion)
			case "API Gateway":
				m.loadingMsg = "Connecting to API Gateway…"
				m.screen = int(ScreenLoading)
				return m, createAPIGatewayClientCmd(m.selectedProfile, m.selectedRegion)
			}
		case "esc", "backspace":
			m.screen = int(ScreenProfiles)
			return m, nil
		case "q":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.serviceList, cmd = m.serviceList.Update(msg)
	return m, cmd
}

func (m Model) updatePools(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.poolList.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.poolList, cmd = m.poolList.Update(msg)
			return m, cmd
		}
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
			m.screen = int(ScreenServiceSelect)
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
	m.statusMessage = ""
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
			wasAction := m.pendingAction
			m.pendingAction = ActionNone
			if wasAction == ActionDynamoDeleteItem {
				m.screen = int(ScreenDynamoDetail)
			} else {
				m.screen = int(ScreenUserDetail)
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) runPendingAction() (Model, tea.Cmd) {
	m.loadingMsg = "Applying change…"
	m.screen = int(ScreenLoading)
	switch m.pendingAction {
	case ActionEnable:
		return m, enableUserCmd(m.cognitoClient, m.selectedPool.ID, m.selectedUser.Username)
	case ActionDisable:
		return m, disableUserCmd(m.cognitoClient, m.selectedPool.ID, m.selectedUser.Username)
	case ActionDelete:
		return m, deleteUserCmd(m.cognitoClient, m.selectedPool.ID, m.selectedUser.Username)
	case ActionResetPassword:
		return m, resetPasswordCmd(m.cognitoClient, m.selectedPool.ID, m.selectedUser.Username)
	case ActionConfirmUser:
		return m, confirmUserCmd(m.cognitoClient, m.selectedPool.ID, m.selectedUser.Username)
	case ActionSignOut:
		return m, signOutUserCmd(m.cognitoClient, m.selectedPool.ID, m.selectedUser.Username)
	case ActionDynamoDeleteItem:
		key := primaryKeyFrom(*m.selectedDynamoItem, m.selectedDynamoTable.PKName, m.selectedDynamoTable.SKName)
		return m, deleteDynamoItemCmd(m.dynamoClient, m.selectedDynamoTable.Name, key)
	}
	return m, nil
}

func (m Model) updateDynamoTables(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.dynamoTableList.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.dynamoTableList, cmd = m.dynamoTableList.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "enter", " ":
			if item, ok := m.dynamoTableList.SelectedItem().(dynamoTableItem); ok {
				t := item.table
				m.selectedDynamoTable = &t
				m.dynamoActiveFilter = ""
				m.dynamoItems = nil
				m.dynamoNextKey = nil
				m.loadingMsg = fmt.Sprintf("Scanning %s…", t.Name)
				m.screen = int(ScreenLoading)
				return m, scanDynamoItemsCmd(m.dynamoClient, t.Name, "", nil, false)
			}
		case "esc", "backspace":
			m.screen = int(ScreenServiceSelect)
			return m, nil
		case "r":
			m.loadingMsg = "Refreshing tables…"
			m.screen = int(ScreenLoading)
			return m, loadDynamoTablesCmd(m.dynamoClient)
		case "q":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.dynamoTableList, cmd = m.dynamoTableList.Update(msg)
	return m, cmd
}

func (m Model) updateDynamoItems(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.dynamoFilterMode {
			switch msg.String() {
			case "enter":
				m.dynamoActiveFilter = m.dynamoFilterInput.Value()
				m.dynamoFilterMode = false
				m.dynamoFilterInput.Blur()
				m.dynamoItems = nil
				m.dynamoNextKey = nil
				m.loadingMsg = "Scanning with filter…"
				m.screen = int(ScreenLoading)
				return m, scanDynamoItemsCmd(m.dynamoClient, m.selectedDynamoTable.Name, m.dynamoActiveFilter, nil, false)
			case "esc":
				m.dynamoFilterMode = false
				m.dynamoFilterInput.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.dynamoFilterInput, cmd = m.dynamoFilterInput.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "enter":
			idx := m.dynamoItemsTable.Cursor()
			if idx >= 0 && idx < len(m.dynamoItems) {
				item := m.dynamoItems[idx]
				m.selectedDynamoItem = &item
				m.dynamoDetailCursor = 0
				m.statusMessage = ""
				m.screen = int(ScreenDynamoDetail)
			}
		case "esc", "backspace":
			m.screen = int(ScreenDynamoTables)
			return m, nil
		case "/":
			m.dynamoFilterMode = true
			m.dynamoFilterInput.Focus()
			return m, nil
		case "c":
			m.dynamoActiveFilter = ""
			m.dynamoFilterInput.SetValue("")
			m.dynamoItems = nil
			m.dynamoNextKey = nil
			m.loadingMsg = "Scanning…"
			m.screen = int(ScreenLoading)
			return m, scanDynamoItemsCmd(m.dynamoClient, m.selectedDynamoTable.Name, "", nil, false)
		case "n":
			if m.dynamoNextKey != nil {
				m.loadingMsg = "Loading more items…"
				m.screen = int(ScreenLoading)
				return m, scanDynamoItemsCmd(m.dynamoClient, m.selectedDynamoTable.Name, m.dynamoActiveFilter, m.dynamoNextKey, true)
			}
		case "r":
			m.dynamoItems = nil
			m.dynamoNextKey = nil
			m.loadingMsg = "Refreshing items…"
			m.screen = int(ScreenLoading)
			return m, scanDynamoItemsCmd(m.dynamoClient, m.selectedDynamoTable.Name, m.dynamoActiveFilter, nil, false)
		case "q":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.dynamoItemsTable, cmd = m.dynamoItemsTable.Update(msg)
	return m, cmd
}

func (m Model) updateDynamoDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			m.statusMessage = ""
			m.screen = int(ScreenDynamoItems)
			return m, nil
		case "d":
			m.pendingAction = ActionDynamoDeleteItem
			m.confirmMsg = "⚠  Delete this item?\nThis cannot be undone."
			m.screen = int(ScreenConfirm)
			return m, nil
		case "up", "k":
			if m.selectedDynamoItem != nil && m.dynamoDetailCursor > 0 {
				m.dynamoDetailCursor--
			}
		case "down", "j":
			if m.selectedDynamoItem != nil && m.dynamoDetailCursor < len(m.selectedDynamoItem.Attrs)-1 {
				m.dynamoDetailCursor++
			}
		case "q":
			return m, tea.Quit
		}
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

func (m Model) updateLambdaFunctions(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.lambdaList.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.lambdaList, cmd = m.lambdaList.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "enter", " ":
			if item, ok := m.lambdaList.SelectedItem().(lambdaFunctionItem); ok {
				fn := item.fn
				m.selectedLambda = &fn
				m.lambdaLogStreams = nil
				m.loadingMsg = fmt.Sprintf("Loading log streams for %s…", fn.Name)
				m.screen = int(ScreenLoading)
				return m, loadLambdaLogStreamsCmd(m.lambdaClient, fn.LogGroup)
			}
		case "esc", "backspace":
			m.screen = int(ScreenServiceSelect)
			return m, nil
		case "r":
			m.loadingMsg = "Refreshing Lambda functions…"
			m.screen = int(ScreenLoading)
			return m, loadLambdaFunctionsCmd(m.lambdaClient)
		case "q":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.lambdaList, cmd = m.lambdaList.Update(msg)
	return m, cmd
}

func (m Model) updateLambdaLogStreams(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.lambdaStreamList.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.lambdaStreamList, cmd = m.lambdaStreamList.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "enter", " ":
			if item, ok := m.lambdaStreamList.SelectedItem().(lambdaLogStreamItem); ok {
				s := item.stream
				m.selectedLogStream = &s
				m.lambdaLogEvents = nil
				m.loadingMsg = "Loading log events…"
				m.screen = int(ScreenLoading)
				return m, loadLambdaLogEventsCmd(m.lambdaClient, m.selectedLambda.LogGroup, s.Name)
			}
		case "esc", "backspace":
			m.screen = int(ScreenLambdaFunctions)
			return m, nil
		case "r":
			if m.selectedLambda != nil {
				m.loadingMsg = "Refreshing log streams…"
				m.screen = int(ScreenLoading)
				return m, loadLambdaLogStreamsCmd(m.lambdaClient, m.selectedLambda.LogGroup)
			}
		case "q":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.lambdaStreamList, cmd = m.lambdaStreamList.Update(msg)
	return m, cmd
}

func (m Model) updateLambdaLogs(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			m.screen = int(ScreenLambdaLogStreams)
			return m, nil
		case "r":
			if m.selectedLambda != nil && m.selectedLogStream != nil {
				m.loadingMsg = "Refreshing log events…"
				m.screen = int(ScreenLoading)
				return m, loadLambdaLogEventsCmd(m.lambdaClient, m.selectedLambda.LogGroup, m.selectedLogStream.Name)
			}
		case "g":
			m.lambdaLogsViewport.GotoTop()
			return m, nil
		case "G":
			m.lambdaLogsViewport.GotoBottom()
			return m, nil
		case "q":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.lambdaLogsViewport, cmd = m.lambdaLogsViewport.Update(msg)
	return m, cmd
}

// formatLogEvents renders log events as a styled string for the viewport.
func formatLogEvents(events []LambdaLogEvent, width int) string {
	if len(events) == 0 {
		return dimStyle.Render("  (no log events in this stream)")
	}
	var b strings.Builder
	for _, e := range events {
		ts := dimStyle.Render(e.Timestamp.Format("2006-01-02 15:04:05.000"))
		msg := strings.TrimRight(e.Message, "\r\n")
		// Indent continuation lines to align with message start.
		lines := strings.Split(msg, "\n")
		first := true
		for _, line := range lines {
			if first {
				b.WriteString(ts + "  " + lipgloss.NewStyle().Width(width-30).Render(line) + "\n")
				first = false
			} else {
				b.WriteString(strings.Repeat(" ", 26) + line + "\n")
			}
		}
	}
	return b.String()
}

func (m Model) updateAPIGateways(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.apigwList.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.apigwList, cmd = m.apigwList.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "enter", " ":
			if item, ok := m.apigwList.SelectedItem().(apigwAPIItem); ok {
				api := item.api
				m.selectedAPI = &api
				m.loadingMsg = fmt.Sprintf("Loading %s…", api.Name)
				m.screen = int(ScreenLoading)
				return m, loadAPIGatewayDetailCmd(m.apigwClient, api)
			}
		case "esc", "backspace":
			m.screen = int(ScreenServiceSelect)
			return m, nil
		case "r":
			m.loadingMsg = "Refreshing APIs…"
			m.screen = int(ScreenLoading)
			return m, loadAPIGatewayAPIsCmd(m.apigwClient)
		case "q":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.apigwList, cmd = m.apigwList.Update(msg)
	return m, cmd
}

func (m Model) updateAPIDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.apigwRouteList.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.apigwRouteList, cmd = m.apigwRouteList.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "esc", "backspace":
			m.screen = int(ScreenAPIGateways)
			return m, nil
		case "r":
			if m.selectedAPI != nil {
				m.loadingMsg = fmt.Sprintf("Refreshing %s…", m.selectedAPI.Name)
				m.screen = int(ScreenLoading)
				return m, loadAPIGatewayDetailCmd(m.apigwClient, *m.selectedAPI)
			}
		case "q":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.apigwRouteList, cmd = m.apigwRouteList.Update(msg)
	return m, cmd
}
