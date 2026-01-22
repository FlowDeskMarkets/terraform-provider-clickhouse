package sdk

import (
	"context"
	"fmt"

	"github.com/FlowdeskMarkets/terraform-provider-clickhouse/pkg/models"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (c *Client) GetColumnDefintions(columns []models.ColumnDefinition) []map[string]interface{} {
	var ret []map[string]interface{}

	for _, column := range columns {
		ret = append(ret, map[string]interface{}{
			"name":               column.Name,
			"type":               column.Type,
			"comment":            column.Comment,
			"default_kind":       column.DefaultKind,
			"default_expression": column.DefaultExpression,
			"compression_codec":  column.CompressionCodec,
		})
	}
	return ret
}

func (c *Client) getColumns(ctx context.Context, database string, table string) ([]models.CHColumn, error) {
	query := fmt.Sprintf(
		"SELECT database, table, name, type, comment, default_kind, default_expression, compression_codec FROM system.columns WHERE database = '%s' AND table = '%s'",
		database,
		table,
	)
	rows, err := c.Conn.Query(ctx, query)

	if err != nil {
		return nil, fmt.Errorf("reading columns from Clickhouse: %v", err)
	}

	var chColumns []models.CHColumn
	for rows.Next() {
		var column models.CHColumn
		err := rows.ScanStruct(&column)
		if err != nil {
			return nil, fmt.Errorf("scanning Clickhouse column row: %v", err)
		}
		chColumns = append(chColumns, column)
	}
	return chColumns, nil
}

func copyToMap(iface interface{}) map[string]interface{} {
	mapCopy := iface.(map[string]interface{})
	mapNew := make(map[string]interface{})
	for k, v := range mapCopy {
		mapNew[k] = v
	}
	return mapNew
}

func executeQuery(ctx context.Context, c *Client, query string) error {
	err := c.Conn.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("executing query: %v", err)
	}
	return nil
}

func createColumnsMap(columns []interface{}) map[string]map[string]interface{} {
	columnsMap := make(map[string]map[string]interface{})
	for _, column := range columns {
		columnMap := column.(map[string]interface{})
		columnName := columnMap["name"].(string)
		columnsMap[columnName] = columnMap
	}
	return columnsMap
}

func UpdateColumns(ctx context.Context, c *Client, table models.TableResource, clusterStatement string, columnMap map[string]interface{}, oldColumnsMap map[string]map[string]interface{}) error {
	columnName := columnMap["name"].(string)
	oldColumnMap, exists := oldColumnsMap[columnName]

	generateArgs := func(extraArgs ...interface{}) []interface{} {
		return append([]interface{}{table.Database, table.Name, clusterStatement, columnName}, extraArgs...)
	}

	changes := []struct {
		condition bool
		query     string
		args      []interface{}
	}{
		{
			condition: !exists,
			query:     "ALTER TABLE %s.%s %s ADD COLUMN %s %s %s %s %s %s %s",
			args:      generateArgs(columnMap["type"], columnMap["default_kind"], columnMap["default_expression"], columnMap["compression_codec"], getComment(columnMap["comment"].(string)), columnMap["location"]),
		},
		{
			condition: exists && columnDiffers(oldColumnMap, columnMap, "type"),
			query:     "ALTER TABLE %s.%s %s MODIFY COLUMN %s %s",
			args:      generateArgs(columnMap["type"]),
		},
		{
			condition: exists && columnDiffers(oldColumnMap, columnMap, "comment"),
			query:     "ALTER TABLE %s.%s %s COMMENT COLUMN %s '%s'",
			args:      generateArgs(columnMap["comment"]),
		},
		{
			condition: exists && columnDiffers(oldColumnMap, columnMap, "default_kind", "default_expression", "compression_codec"),
			query:     "ALTER TABLE %s.%s %s MODIFY COLUMN %s %s %s %s",
			args: generateArgs(
				columnMap["default_kind"],
				columnMap["default_expression"],
				columnMap["compression_codec"],
			),
		},
	}

	for _, change := range changes {
		if change.condition {
			query := fmt.Sprintf(change.query, change.args...)
			tflog.Debug(ctx, fmt.Sprintf("Executing query: %s", query))

			if err := executeQuery(ctx, c, query); err != nil {
				return fmt.Errorf("failed to modify column %s: %w", columnName, err)
			}
		}
	}
	return nil
}

func columnDiffers(oldMap, newMap map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if oldMap[key] != newMap[key] {
			return true
		}
	}
	return false
}

func dropOldColumns(ctx context.Context, c *Client, table models.TableResource, clusterStatement string, oldColumns []interface{}, newColumnsMap map[string]map[string]interface{}) error {
	for _, column := range oldColumns {
		columnMap := column.(map[string]interface{})
		if _, exists := newColumnsMap[columnMap["name"].(string)]; !exists {
			err := executeQuery(ctx, c, fmt.Sprintf(
				"ALTER TABLE %s.%s %s DROP COLUMN %s",
				table.Database, table.Name, clusterStatement, columnMap["name"]))
			if err != nil {
				return fmt.Errorf("dropping columns from Clickhouse table: %v", err)
			}
		}
	}
	return nil
}
