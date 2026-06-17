package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	tea "github.com/charmbracelet/bubbletea"
)

// ── Types ─────────────────────────────────────────────────────────────────────

type LambdaFunction struct {
	Name         string
	Runtime      string
	Handler      string
	LastModified string
	State        string
	MemorySize   int32
	Timeout      int32
	Description  string
	LogGroup     string
}

type LambdaLogStream struct {
	Name          string
	LastEventTime time.Time
	CreationTime  time.Time
}

type LambdaLogEvent struct {
	Timestamp time.Time
	Message   string
}

// LambdaClient wraps the Lambda and CloudWatch Logs service clients.
type LambdaClient struct {
	lambdaSvc *lambda.Client
	logsSvc   *cwlogs.Client
	Region    string
}

func NewLambdaClient(profile, region string) (*LambdaClient, error) {
	cfg, err := loadAWSConfig(profile, region)
	if err != nil {
		return nil, err
	}
	return &LambdaClient{
		lambdaSvc: lambda.NewFromConfig(cfg),
		logsSvc:   cwlogs.NewFromConfig(cfg),
		Region:    cfg.Region,
	}, nil
}

// ListFunctions returns all Lambda functions sorted by name.
func (c *LambdaClient) ListFunctions() ([]LambdaFunction, error) {
	var fns []LambdaFunction
	var marker *string
	for {
		out, err := c.lambdaSvc.ListFunctions(context.Background(), &lambda.ListFunctionsInput{
			MaxItems: aws.Int32(50),
			Marker:   marker,
		})
		if err != nil {
			return nil, fmt.Errorf("list functions: %w", err)
		}
		for _, f := range out.Functions {
			state := string(f.State)
			if state == "" {
				state = "Active"
			}
			fn := LambdaFunction{
				Name:         aws.ToString(f.FunctionName),
				Runtime:      string(f.Runtime),
				Handler:      aws.ToString(f.Handler),
				LastModified: aws.ToString(f.LastModified),
				State:        state,
				Description:  aws.ToString(f.Description),
				LogGroup:     "/aws/lambda/" + aws.ToString(f.FunctionName),
			}
			if f.MemorySize != nil {
				fn.MemorySize = *f.MemorySize
			}
			if f.Timeout != nil {
				fn.Timeout = *f.Timeout
			}
			fns = append(fns, fn)
		}
		if out.NextMarker == nil {
			break
		}
		marker = out.NextMarker
	}
	sort.Slice(fns, func(i, j int) bool { return fns[i].Name < fns[j].Name })
	return fns, nil
}

// ListLogStreams returns the most-recent 20 CloudWatch log streams for the log group.
func (c *LambdaClient) ListLogStreams(logGroup string) ([]LambdaLogStream, error) {
	out, err := c.logsSvc.DescribeLogStreams(context.Background(), &cwlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(logGroup),
		Descending:   aws.Bool(true),
		OrderBy:      cwlogstypes.OrderByLastEventTime,
		Limit:        aws.Int32(20),
	})
	if err != nil {
		return nil, fmt.Errorf("describe log streams: %w", err)
	}
	streams := make([]LambdaLogStream, 0, len(out.LogStreams))
	for _, s := range out.LogStreams {
		ls := LambdaLogStream{Name: aws.ToString(s.LogStreamName)}
		if s.LastEventTimestamp != nil {
			ls.LastEventTime = time.UnixMilli(*s.LastEventTimestamp)
		}
		if s.CreationTime != nil {
			ls.CreationTime = time.UnixMilli(*s.CreationTime)
		}
		streams = append(streams, ls)
	}
	return streams, nil
}

// GetLogEvents fetches all log events from a single stream (up to 500 per request).
func (c *LambdaClient) GetLogEvents(logGroup, logStream string) ([]LambdaLogEvent, error) {
	var events []LambdaLogEvent
	var nextToken *string
	for {
		out, err := c.logsSvc.GetLogEvents(context.Background(), &cwlogs.GetLogEventsInput{
			LogGroupName:  aws.String(logGroup),
			LogStreamName: aws.String(logStream),
			StartFromHead: aws.Bool(true),
			Limit:         aws.Int32(500),
			NextToken:     nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("get log events: %w", err)
		}
		for _, e := range out.Events {
			ev := LambdaLogEvent{Message: aws.ToString(e.Message)}
			if e.Timestamp != nil {
				ev.Timestamp = time.UnixMilli(*e.Timestamp)
			}
			events = append(events, ev)
		}
		// GetLogEvents returns the same nextForwardToken when no more events remain.
		if out.NextForwardToken == nil || (nextToken != nil && *out.NextForwardToken == *nextToken) {
			break
		}
		nextToken = out.NextForwardToken
	}
	return events, nil
}

// ── Tea messages ──────────────────────────────────────────────────────────────

type lambdaClientReadyMsg struct{ client *LambdaClient }
type lambdaClientErrMsg struct{ err error }
type lambdaFunctionsLoadedMsg struct{ fns []LambdaFunction }
type lambdaFunctionsErrMsg struct{ err error }
type lambdaLogStreamsLoadedMsg struct{ streams []LambdaLogStream }
type lambdaLogStreamsErrMsg struct{ err error }
type lambdaLogEventsLoadedMsg struct{ events []LambdaLogEvent }
type lambdaLogEventsErrMsg struct{ err error }

// ── Tea commands ──────────────────────────────────────────────────────────────

func createLambdaClientCmd(profile, region string) tea.Cmd {
	return func() tea.Msg {
		c, err := NewLambdaClient(profile, region)
		if err != nil {
			return lambdaClientErrMsg{err}
		}
		return lambdaClientReadyMsg{c}
	}
}

func loadLambdaFunctionsCmd(c *LambdaClient) tea.Cmd {
	return func() tea.Msg {
		fns, err := c.ListFunctions()
		if err != nil {
			return lambdaFunctionsErrMsg{err}
		}
		return lambdaFunctionsLoadedMsg{fns}
	}
}

func loadLambdaLogStreamsCmd(c *LambdaClient, logGroup string) tea.Cmd {
	return func() tea.Msg {
		streams, err := c.ListLogStreams(logGroup)
		if err != nil {
			return lambdaLogStreamsErrMsg{err}
		}
		return lambdaLogStreamsLoadedMsg{streams}
	}
}

func loadLambdaLogEventsCmd(c *LambdaClient, logGroup, logStream string) tea.Cmd {
	return func() tea.Msg {
		events, err := c.GetLogEvents(logGroup, logStream)
		if err != nil {
			return lambdaLogEventsErrMsg{err}
		}
		return lambdaLogEventsLoadedMsg{events}
	}
}
