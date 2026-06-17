package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigwv1 "github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwv2 "github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	tea "github.com/charmbracelet/bubbletea"
)

// ── Types ─────────────────────────────────────────────────────────────────────

type APIGatewayAPI struct {
	ID          string
	Name        string
	Type        string // "REST", "HTTP", "WebSocket"
	Description string
	CreatedAt   time.Time
	Endpoint    string
}

type APIGatewayStage struct {
	Name        string
	Description string
	InvokeURL   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type APIGatewayRoute struct {
	Key    string // e.g. "GET /users" or "POST /items/{id}"
	Target string // integration target (if available)
}

type APIGatewayDetail struct {
	API    APIGatewayAPI
	Stages []APIGatewayStage
	Routes []APIGatewayRoute
}

// APIGatewayClient wraps both the V1 (REST) and V2 (HTTP/WebSocket) clients.
type APIGatewayClient struct {
	v1     *apigwv1.Client
	v2     *apigwv2.Client
	Region string
}

func NewAPIGatewayClient(profile, region string) (*APIGatewayClient, error) {
	cfg, err := loadAWSConfig(profile, region)
	if err != nil {
		return nil, err
	}
	return &APIGatewayClient{
		v1:     apigwv1.NewFromConfig(cfg),
		v2:     apigwv2.NewFromConfig(cfg),
		Region: cfg.Region,
	}, nil
}

// ListAPIs returns all REST, HTTP and WebSocket APIs sorted by name.
func (c *APIGatewayClient) ListAPIs() ([]APIGatewayAPI, error) {
	var apis []APIGatewayAPI

	// ── V1: REST APIs ──────────────────────────────────────────────────────
	var pos *string
	for {
		out, err := c.v1.GetRestApis(context.Background(), &apigwv1.GetRestApisInput{
			Limit:    aws.Int32(100),
			Position: pos,
		})
		if err != nil {
			return nil, fmt.Errorf("get rest apis: %w", err)
		}
		for _, a := range out.Items {
			api := APIGatewayAPI{
				ID:          aws.ToString(a.Id),
				Name:        aws.ToString(a.Name),
				Type:        "REST",
				Description: aws.ToString(a.Description),
			}
			if a.CreatedDate != nil {
				api.CreatedAt = *a.CreatedDate
			}
			apis = append(apis, api)
		}
		if out.Position == nil {
			break
		}
		pos = out.Position
	}

	// ── V2: HTTP + WebSocket APIs ──────────────────────────────────────────
	var next *string
	for {
		out, err := c.v2.GetApis(context.Background(), &apigwv2.GetApisInput{
			NextToken: next,
		})
		if err != nil {
			return nil, fmt.Errorf("get http apis: %w", err)
		}
		for _, a := range out.Items {
			api := APIGatewayAPI{
				ID:          aws.ToString(a.ApiId),
				Name:        aws.ToString(a.Name),
				Type:        string(a.ProtocolType),
				Description: aws.ToString(a.Description),
				Endpoint:    aws.ToString(a.ApiEndpoint),
			}
			if a.CreatedDate != nil {
				api.CreatedAt = *a.CreatedDate
			}
			apis = append(apis, api)
		}
		if out.NextToken == nil {
			break
		}
		next = out.NextToken
	}

	sort.Slice(apis, func(i, j int) bool { return apis[i].Name < apis[j].Name })
	return apis, nil
}

// GetAPIDetail fetches stages and routes/resources for the given API.
func (c *APIGatewayClient) GetAPIDetail(api APIGatewayAPI) (*APIGatewayDetail, error) {
	detail := &APIGatewayDetail{API: api}
	var err error

	switch api.Type {
	case "REST":
		detail.Stages, err = c.getRESTStages(api)
		if err != nil {
			return nil, err
		}
		detail.Routes, err = c.getRESTRoutes(api)
		if err != nil {
			return nil, err
		}
	case "HTTP", "WEBSOCKET":
		detail.Stages, err = c.getV2Stages(api)
		if err != nil {
			return nil, err
		}
		detail.Routes, err = c.getV2Routes(api)
		if err != nil {
			return nil, err
		}
	}

	return detail, nil
}

func (c *APIGatewayClient) getRESTStages(api APIGatewayAPI) ([]APIGatewayStage, error) {
	out, err := c.v1.GetStages(context.Background(), &apigwv1.GetStagesInput{
		RestApiId: aws.String(api.ID),
	})
	if err != nil {
		return nil, fmt.Errorf("get rest stages: %w", err)
	}
	stages := make([]APIGatewayStage, 0, len(out.Item))
	for _, s := range out.Item {
		name := aws.ToString(s.StageName)
		st := APIGatewayStage{
			Name:        name,
			Description: aws.ToString(s.Description),
			InvokeURL:   fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/%s", api.ID, c.Region, name),
		}
		if s.CreatedDate != nil {
			st.CreatedAt = *s.CreatedDate
		}
		if s.LastUpdatedDate != nil {
			st.UpdatedAt = *s.LastUpdatedDate
		}
		stages = append(stages, st)
	}
	return stages, nil
}

func (c *APIGatewayClient) getRESTRoutes(api APIGatewayAPI) ([]APIGatewayRoute, error) {
	var routes []APIGatewayRoute
	var pos *string
	for {
		out, err := c.v1.GetResources(context.Background(), &apigwv1.GetResourcesInput{
			RestApiId: aws.String(api.ID),
			Embed:     []string{"methods"},
			Limit:     aws.Int32(100),
			Position:  pos,
		})
		if err != nil {
			return nil, fmt.Errorf("get rest resources: %w", err)
		}
		for _, r := range out.Items {
			path := aws.ToString(r.Path)
			if len(r.ResourceMethods) == 0 {
				continue
			}
			methods := make([]string, 0, len(r.ResourceMethods))
			for method := range r.ResourceMethods {
				methods = append(methods, method)
			}
			sort.Strings(methods)
			routes = append(routes, APIGatewayRoute{
				Key: strings.Join(methods, ", ") + "  " + path,
			})
		}
		if out.Position == nil {
			break
		}
		pos = out.Position
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].Key < routes[j].Key })
	return routes, nil
}

func (c *APIGatewayClient) getV2Stages(api APIGatewayAPI) ([]APIGatewayStage, error) {
	var stages []APIGatewayStage
	var next *string
	for {
		out, err := c.v2.GetStages(context.Background(), &apigwv2.GetStagesInput{
			ApiId:     aws.String(api.ID),
			NextToken: next,
		})
		if err != nil {
			return nil, fmt.Errorf("get v2 stages: %w", err)
		}
		for _, s := range out.Items {
			name := aws.ToString(s.StageName)
			stagePath := name
			if stagePath == "$default" {
				stagePath = ""
			}
			invokeURL := api.Endpoint
			if stagePath != "" {
				invokeURL = api.Endpoint + "/" + stagePath
			}
			st := APIGatewayStage{
				Name:        name,
				Description: aws.ToString(s.Description),
				InvokeURL:   invokeURL,
			}
			if s.CreatedDate != nil {
				st.CreatedAt = *s.CreatedDate
			}
			if s.LastUpdatedDate != nil {
				st.UpdatedAt = *s.LastUpdatedDate
			}
			stages = append(stages, st)
		}
		if out.NextToken == nil {
			break
		}
		next = out.NextToken
	}
	return stages, nil
}

func (c *APIGatewayClient) getV2Routes(api APIGatewayAPI) ([]APIGatewayRoute, error) {
	var routes []APIGatewayRoute
	var next *string
	for {
		out, err := c.v2.GetRoutes(context.Background(), &apigwv2.GetRoutesInput{
			ApiId:     aws.String(api.ID),
			NextToken: next,
		})
		if err != nil {
			return nil, fmt.Errorf("get v2 routes: %w", err)
		}
		for _, r := range out.Items {
			routes = append(routes, APIGatewayRoute{
				Key:    aws.ToString(r.RouteKey),
				Target: aws.ToString(r.Target),
			})
		}
		if out.NextToken == nil {
			break
		}
		next = out.NextToken
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].Key < routes[j].Key })
	return routes, nil
}

// ── Tea messages ──────────────────────────────────────────────────────────────

type apigwClientReadyMsg struct{ client *APIGatewayClient }
type apigwClientErrMsg struct{ err error }
type apigwAPIsLoadedMsg struct{ apis []APIGatewayAPI }
type apigwAPIsErrMsg struct{ err error }
type apigwDetailLoadedMsg struct{ detail *APIGatewayDetail }
type apigwDetailErrMsg struct{ err error }

// ── Tea commands ──────────────────────────────────────────────────────────────

func createAPIGatewayClientCmd(profile, region string) tea.Cmd {
	return func() tea.Msg {
		c, err := NewAPIGatewayClient(profile, region)
		if err != nil {
			return apigwClientErrMsg{err}
		}
		return apigwClientReadyMsg{c}
	}
}

func loadAPIGatewayAPIsCmd(c *APIGatewayClient) tea.Cmd {
	return func() tea.Msg {
		apis, err := c.ListAPIs()
		if err != nil {
			return apigwAPIsErrMsg{err}
		}
		return apigwAPIsLoadedMsg{apis}
	}
}

func loadAPIGatewayDetailCmd(c *APIGatewayClient, api APIGatewayAPI) tea.Cmd {
	return func() tea.Msg {
		detail, err := c.GetAPIDetail(api)
		if err != nil {
			return apigwDetailErrMsg{err}
		}
		return apigwDetailLoadedMsg{detail}
	}
}
