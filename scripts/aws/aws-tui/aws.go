package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	tea "github.com/charmbracelet/bubbletea"
)

// PoolItem represents a Cognito user pool.
type PoolItem struct {
	ID   string
	Name string
}

// UserItem is a summary of a Cognito user (from ListUsers).
type UserItem struct {
	Username    string
	Status      string
	Enabled     bool
	Email       string
	CreatedDate time.Time
	Attributes  map[string]string
}

// UserDetail extends UserItem with groups and MFA info (from AdminGetUser).
type UserDetail struct {
	UserItem
	Groups     []string
	MFAOptions []string
}

// CognitoClient wraps the AWS Cognito Identity Provider client.
type CognitoClient struct {
	svc    *cognitoidentityprovider.Client
	Region string
}

// loadAWSConfig is the shared config loader used by all service clients.
func loadAWSConfig(profile, region string) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithSharedConfigProfile(profile),
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return cfg, nil
}

// NewCognitoClient creates an authenticated client for the given profile and region.
func NewCognitoClient(profile, region string) (*CognitoClient, error) {
	cfg, err := loadAWSConfig(profile, region)
	if err != nil {
		return nil, err
	}
	return &CognitoClient{
		svc:    cognitoidentityprovider.NewFromConfig(cfg),
		Region: cfg.Region,
	}, nil
}

// GetProfiles reads profile names from ~/.aws/config and ~/.aws/credentials.
func GetProfiles() ([]string, error) {
	home, _ := os.UserHomeDir()
	profiles := map[string]bool{}

	readSections := func(path string, trimProfilePrefix bool) {
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
				continue
			}
			name := line[1 : len(line)-1]
			if trimProfilePrefix {
				name = strings.TrimPrefix(name, "profile ")
			}
			profiles[name] = true
		}
	}

	readSections(filepath.Join(home, ".aws", "config"), true)
	readSections(filepath.Join(home, ".aws", "credentials"), false)

	if len(profiles) == 0 {
		return nil, fmt.Errorf("no AWS profiles found in ~/.aws/config or ~/.aws/credentials")
	}

	result := make([]string, 0, len(profiles))
	for p := range profiles {
		result = append(result, p)
	}
	sort.Strings(result)

	// Move "default" to the top.
	for i, p := range result {
		if p == "default" {
			result = append([]string{"default"}, append(result[:i], result[i+1:]...)...)
			break
		}
	}
	return result, nil
}

// GetProfileRegion returns the configured region for a profile, or "".
func GetProfileRegion(profile string) string {
	home, _ := os.UserHomeDir()
	f, err := os.Open(filepath.Join(home, ".aws", "config"))
	if err != nil {
		return ""
	}
	defer f.Close()

	var inSection bool
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimPrefix(line[1:len(line)-1], "profile ")
			inSection = name == profile
		} else if inSection && strings.HasPrefix(line, "region") {
			if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// ListUserPools returns all Cognito user pools (follows pagination automatically).
func (c *CognitoClient) ListUserPools() ([]PoolItem, error) {
	var pools []PoolItem
	var nextToken *string
	for {
		out, err := c.svc.ListUserPools(context.Background(), &cognitoidentityprovider.ListUserPoolsInput{
			MaxResults: aws.Int32(60),
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list user pools: %w", err)
		}
		for _, p := range out.UserPools {
			pools = append(pools, PoolItem{ID: aws.ToString(p.Id), Name: aws.ToString(p.Name)})
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return pools, nil
}

// ListUsers returns up to 60 users matching filter; paginationToken continues a previous query.
func (c *CognitoClient) ListUsers(poolID, filter, paginationToken string) ([]UserItem, string, error) {
	input := &cognitoidentityprovider.ListUsersInput{
		UserPoolId: aws.String(poolID),
		Limit:      aws.Int32(60),
	}
	if filter != "" {
		input.Filter = aws.String(filter)
	}
	if paginationToken != "" {
		input.PaginationToken = aws.String(paginationToken)
	}

	out, err := c.svc.ListUsers(context.Background(), input)
	if err != nil {
		return nil, "", fmt.Errorf("list users: %w", err)
	}

	users := make([]UserItem, 0, len(out.Users))
	for _, u := range out.Users {
		item := UserItem{
			Username:   aws.ToString(u.Username),
			Enabled:    u.Enabled,
			Status:     string(u.UserStatus),
			Attributes: make(map[string]string),
		}
		if u.UserCreateDate != nil {
			item.CreatedDate = *u.UserCreateDate
		}
		for _, a := range u.Attributes {
			if a.Name != nil && a.Value != nil {
				item.Attributes[*a.Name] = *a.Value
			}
		}
		item.Email = item.Attributes["email"]
		users = append(users, item)
	}

	var next string
	if out.PaginationToken != nil {
		next = *out.PaginationToken
	}
	return users, next, nil
}

// GetUser fetches full details (attributes + groups) via AdminGetUser.
func (c *CognitoClient) GetUser(poolID, username string) (*UserDetail, error) {
	out, err := c.svc.AdminGetUser(context.Background(), &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(poolID),
		Username:   aws.String(username),
	})
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	d := &UserDetail{}
	d.Username = aws.ToString(out.Username)
	d.Enabled = out.Enabled
	d.Status = string(out.UserStatus)
	d.Attributes = make(map[string]string)
	if out.UserCreateDate != nil {
		d.CreatedDate = *out.UserCreateDate
	}
	for _, a := range out.UserAttributes {
		if a.Name != nil && a.Value != nil {
			d.Attributes[*a.Name] = *a.Value
		}
	}
	d.Email = d.Attributes["email"]

	for _, mfa := range out.MFAOptions {
		d.MFAOptions = append(d.MFAOptions, string(mfa.DeliveryMedium))
	}

	gOut, err := c.svc.AdminListGroupsForUser(context.Background(), &cognitoidentityprovider.AdminListGroupsForUserInput{
		UserPoolId: aws.String(poolID),
		Username:   aws.String(username),
	})
	if err == nil {
		for _, g := range gOut.Groups {
			d.Groups = append(d.Groups, aws.ToString(g.GroupName))
		}
	}

	return d, nil
}

func (c *CognitoClient) EnableUser(poolID, username string) error {
	_, err := c.svc.AdminEnableUser(context.Background(), &cognitoidentityprovider.AdminEnableUserInput{
		UserPoolId: aws.String(poolID), Username: aws.String(username),
	})
	return err
}

func (c *CognitoClient) DisableUser(poolID, username string) error {
	_, err := c.svc.AdminDisableUser(context.Background(), &cognitoidentityprovider.AdminDisableUserInput{
		UserPoolId: aws.String(poolID), Username: aws.String(username),
	})
	return err
}

func (c *CognitoClient) DeleteUser(poolID, username string) error {
	_, err := c.svc.AdminDeleteUser(context.Background(), &cognitoidentityprovider.AdminDeleteUserInput{
		UserPoolId: aws.String(poolID), Username: aws.String(username),
	})
	return err
}

func (c *CognitoClient) ResetPassword(poolID, username string) error {
	_, err := c.svc.AdminResetUserPassword(context.Background(), &cognitoidentityprovider.AdminResetUserPasswordInput{
		UserPoolId: aws.String(poolID), Username: aws.String(username),
	})
	return err
}

func (c *CognitoClient) ConfirmUser(poolID, username string) error {
	_, err := c.svc.AdminConfirmSignUp(context.Background(), &cognitoidentityprovider.AdminConfirmSignUpInput{
		UserPoolId: aws.String(poolID), Username: aws.String(username),
	})
	return err
}

func (c *CognitoClient) SignOutUser(poolID, username string) error {
	_, err := c.svc.AdminUserGlobalSignOut(context.Background(), &cognitoidentityprovider.AdminUserGlobalSignOutInput{
		UserPoolId: aws.String(poolID), Username: aws.String(username),
	})
	return err
}

// ── Tea messages ──────────────────────────────────────────────────────────────

type profilesLoadedMsg struct{ profiles []string }
type profilesErrMsg struct{ err error }
type clientCreatedMsg struct{ client *CognitoClient }
type clientErrMsg struct{ err error }
type poolsLoadedMsg struct{ pools []PoolItem }
type poolsErrMsg struct{ err error }
type usersLoadedMsg struct {
	users     []UserItem
	nextToken string
	append    bool
}
type usersErrMsg struct{ err error }
type userDetailLoadedMsg struct{ detail *UserDetail }
type userDetailErrMsg struct{ err error }
type actionSuccessMsg struct {
	msg    string
	action Action
}
type actionErrMsg struct{ err error }

// ── Tea commands ──────────────────────────────────────────────────────────────

func loadProfilesCmd() tea.Cmd {
	return func() tea.Msg {
		profiles, err := GetProfiles()
		if err != nil {
			return profilesErrMsg{err}
		}
		return profilesLoadedMsg{profiles}
	}
}

func createClientCmd(profile, region string) tea.Cmd {
	return func() tea.Msg {
		c, err := NewCognitoClient(profile, region)
		if err != nil {
			return clientErrMsg{err}
		}
		return clientCreatedMsg{c}
	}
}

func loadPoolsCmd(c *CognitoClient) tea.Cmd {
	return func() tea.Msg {
		pools, err := c.ListUserPools()
		if err != nil {
			return poolsErrMsg{err}
		}
		return poolsLoadedMsg{pools}
	}
}

func loadUsersCmd(c *CognitoClient, poolID, filter, token string, appendMode bool) tea.Cmd {
	return func() tea.Msg {
		users, next, err := c.ListUsers(poolID, filter, token)
		if err != nil {
			return usersErrMsg{err}
		}
		return usersLoadedMsg{users, next, appendMode}
	}
}

func loadUserDetailCmd(c *CognitoClient, poolID, username string) tea.Cmd {
	return func() tea.Msg {
		d, err := c.GetUser(poolID, username)
		if err != nil {
			return userDetailErrMsg{err}
		}
		return userDetailLoadedMsg{d}
	}
}

func enableUserCmd(c *CognitoClient, poolID, username string) tea.Cmd {
	return func() tea.Msg {
		if err := c.EnableUser(poolID, username); err != nil {
			return actionErrMsg{err}
		}
		return actionSuccessMsg{"User enabled", ActionEnable}
	}
}

func disableUserCmd(c *CognitoClient, poolID, username string) tea.Cmd {
	return func() tea.Msg {
		if err := c.DisableUser(poolID, username); err != nil {
			return actionErrMsg{err}
		}
		return actionSuccessMsg{"User disabled", ActionDisable}
	}
}

func deleteUserCmd(c *CognitoClient, poolID, username string) tea.Cmd {
	return func() tea.Msg {
		if err := c.DeleteUser(poolID, username); err != nil {
			return actionErrMsg{err}
		}
		return actionSuccessMsg{"User deleted", ActionDelete}
	}
}

func resetPasswordCmd(c *CognitoClient, poolID, username string) tea.Cmd {
	return func() tea.Msg {
		if err := c.ResetPassword(poolID, username); err != nil {
			return actionErrMsg{err}
		}
		return actionSuccessMsg{"Password reset initiated", ActionResetPassword}
	}
}

func confirmUserCmd(c *CognitoClient, poolID, username string) tea.Cmd {
	return func() tea.Msg {
		if err := c.ConfirmUser(poolID, username); err != nil {
			return actionErrMsg{err}
		}
		return actionSuccessMsg{"User confirmed", ActionConfirmUser}
	}
}

func signOutUserCmd(c *CognitoClient, poolID, username string) tea.Cmd {
	return func() tea.Msg {
		if err := c.SignOutUser(poolID, username); err != nil {
			return actionErrMsg{err}
		}
		return actionSuccessMsg{"User signed out from all devices", ActionSignOut}
	}
}

// buildFilter converts a human-readable search string into a Cognito filter expression.
func buildFilter(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	// Support explicit field:value syntax.
	if idx := strings.Index(q, ":"); idx > 0 {
		field := strings.TrimSpace(q[:idx])
		value := strings.TrimSpace(q[idx+1:])
		return fmt.Sprintf(`%s ^= "%s"`, field, value)
	}
	// email exact match when it looks like one.
	if strings.Contains(q, "@") {
		return fmt.Sprintf(`email = "%s"`, q)
	}
	// Default: username prefix.
	return fmt.Sprintf(`username ^= "%s"`, q)
}
