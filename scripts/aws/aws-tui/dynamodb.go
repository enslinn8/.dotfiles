package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	tea "github.com/charmbracelet/bubbletea"
)

// DynamoTable is a summary of a DynamoDB table.
type DynamoTable struct {
	Name      string
	Status    string
	ItemCount int64
	SizeBytes int64
	PKName    string
	PKType    string
	SKName    string
	SKType    string
}

// DynamoItem is one table row represented as sorted key-value pairs.
type DynamoItem struct {
	Attrs []DynamoAttr
	raw   map[string]types.AttributeValue // kept for deletion
}

// DynamoAttr is one attribute of a DynamoItem.
type DynamoAttr struct {
	Key   string
	Value string
}

// DynamoClient wraps the AWS DynamoDB SDK client.
type DynamoClient struct {
	svc    *dynamodb.Client
	Region string
}

// ListTables returns all table names (paginates automatically).
func (c *DynamoClient) ListTables() ([]string, error) {
	var names []string
	var lastEval *string
	for {
		out, err := c.svc.ListTables(context.Background(), &dynamodb.ListTablesInput{
			ExclusiveStartTableName: lastEval,
		})
		if err != nil {
			return nil, fmt.Errorf("list tables: %w", err)
		}
		names = append(names, out.TableNames...)
		if out.LastEvaluatedTableName == nil {
			break
		}
		lastEval = out.LastEvaluatedTableName
	}
	sort.Strings(names)
	return names, nil
}

// DescribeTable fetches key schema and stats for one table.
func (c *DynamoClient) DescribeTable(name string) (*DynamoTable, error) {
	out, err := c.svc.DescribeTable(context.Background(), &dynamodb.DescribeTableInput{
		TableName: aws.String(name),
	})
	if err != nil {
		return nil, fmt.Errorf("describe table %s: %w", name, err)
	}
	t := out.Table
	dt := &DynamoTable{
		Name:      aws.ToString(t.TableName),
		Status:    string(t.TableStatus),
		ItemCount: aws.ToInt64(t.ItemCount),
		SizeBytes: aws.ToInt64(t.TableSizeBytes),
	}
	attrTypes := map[string]string{}
	for _, ad := range t.AttributeDefinitions {
		attrTypes[aws.ToString(ad.AttributeName)] = string(ad.AttributeType)
	}
	for _, ks := range t.KeySchema {
		n := aws.ToString(ks.AttributeName)
		switch ks.KeyType {
		case types.KeyTypeHash:
			dt.PKName = n
			dt.PKType = attrTypes[n]
		case types.KeyTypeRange:
			dt.SKName = n
			dt.SKType = attrTypes[n]
		}
	}
	return dt, nil
}

// DeleteItem deletes a row by its full primary key.
func (c *DynamoClient) DeleteItem(tableName string, key map[string]types.AttributeValue) error {
	_, err := c.svc.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key:       key,
	})
	return err
}

// marshalDynamoItem converts a raw SDK map into a DynamoItem.
func marshalDynamoItem(raw map[string]types.AttributeValue) DynamoItem {
	var result map[string]interface{}
	_ = attributevalue.UnmarshalMap(raw, &result)
	attrs := make([]DynamoAttr, 0, len(result))
	for k, v := range result {
		attrs = append(attrs, DynamoAttr{Key: k, Value: fmt.Sprintf("%v", v)})
	}
	sort.Slice(attrs, func(i, j int) bool { return attrs[i].Key < attrs[j].Key })
	return DynamoItem{Attrs: attrs, raw: raw}
}

// primaryKeyFrom extracts only the PK (and optional SK) attributes from an item.
func primaryKeyFrom(item DynamoItem, pkName, skName string) map[string]types.AttributeValue {
	key := map[string]types.AttributeValue{}
	if v, ok := item.raw[pkName]; ok {
		key[pkName] = v
	}
	if skName != "" {
		if v, ok := item.raw[skName]; ok {
			key[skName] = v
		}
	}
	return key
}

// ── Tea messages ──────────────────────────────────────────────────────────────

type dynamoClientReadyMsg struct{ client *DynamoClient }
type dynamoTablesLoadedMsg struct{ tables []DynamoTable }
type dynamoTablesErrMsg struct{ err error }
type dynamoItemsLoadedMsg struct {
	items      []DynamoItem
	nextKey    map[string]types.AttributeValue
	appendMode bool
}
type dynamoItemsErrMsg struct{ err error }
type dynamoDeleteSuccessMsg struct{}
type dynamoDeleteErrMsg struct{ err error }

// ── Tea commands ──────────────────────────────────────────────────────────────

func createDynamoClientCmd(profile, region string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := loadAWSConfig(profile, region)
		if err != nil {
			return clientErrMsg{err}
		}
		return dynamoClientReadyMsg{client: &DynamoClient{svc: dynamodb.NewFromConfig(cfg), Region: cfg.Region}}
	}
}

func loadDynamoTablesCmd(c *DynamoClient) tea.Cmd {
	return func() tea.Msg {
		names, err := c.ListTables()
		if err != nil {
			return dynamoTablesErrMsg{err}
		}
		tables := make([]DynamoTable, 0, len(names))
		for _, name := range names {
			dt, err := c.DescribeTable(name)
			if err != nil {
				dt = &DynamoTable{Name: name, Status: "UNKNOWN"}
			}
			tables = append(tables, *dt)
		}
		return dynamoTablesLoadedMsg{tables}
	}
}

func scanDynamoItemsCmd(c *DynamoClient, tableName, filter string, startKey map[string]types.AttributeValue, appendMode bool) tea.Cmd {
	return func() tea.Msg {
		input := &dynamodb.ScanInput{
			TableName: aws.String(tableName),
			Limit:     aws.Int32(50),
		}
		if filter != "" {
			input.FilterExpression = aws.String(filter)
		}
		if startKey != nil {
			input.ExclusiveStartKey = startKey
		}
		out, err := c.svc.Scan(context.Background(), input)
		if err != nil {
			return dynamoItemsErrMsg{fmt.Errorf("scan: %w", err)}
		}
		items := make([]DynamoItem, 0, len(out.Items))
		for _, raw := range out.Items {
			items = append(items, marshalDynamoItem(raw))
		}
		return dynamoItemsLoadedMsg{items: items, nextKey: out.LastEvaluatedKey, appendMode: appendMode}
	}
}

func deleteDynamoItemCmd(c *DynamoClient, tableName string, key map[string]types.AttributeValue) tea.Cmd {
	return func() tea.Msg {
		if err := c.DeleteItem(tableName, key); err != nil {
			return dynamoDeleteErrMsg{err}
		}
		return dynamoDeleteSuccessMsg{}
	}
}
