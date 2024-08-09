package common

import (
	"context"

	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func toTablePhrase(toTable *string) string {
	if toTable != nil {
		return fmt.Sprintf(`,"to_table":"%v"`, *toTable)
	}
	return ""
}

func GetComment(comment string, cluster string, toTable *string) string {
	storingComment := fmt.Sprintf(`{"comment":"%v","cluster":"%v"%s}`, comment, cluster, toTablePhrase(toTable))
	storingComment = strings.Replace(storingComment, "'", "\\'", -1)
	return storingComment
}

func UnmarshalComment(storedComment string) (comment string, cluster string, toTable string, err error) {
	if storedComment == "" {
		return "", "", "", nil
	}
	storedComment = strings.Replace(storedComment, "\\'", "'", -1)

	byteStreamComment := []byte(storedComment)

	var dat map[string]interface{}

	if err := json.Unmarshal(byteStreamComment, &dat); err != nil {
		return "", "", "", err
	}
	comment = dat["comment"].(string)
	cluster = dat["cluster"].(string)
	if dat["to_table"] != nil {
		toTable = dat["to_table"].(string)
	}

	return comment, cluster, toTable, err
}

func GetClusterStatement(cluster string) (clusterStatement string) {
	if cluster != "" {
		return fmt.Sprintf("ON CLUSTER %s", cluster)
	}
	return ""
}

// Quote all strings on a string slice
func Quote(elems []string) []string {
	var quotedElems []string
	for _, elem := range elems {
		quotedElems = append(quotedElems, fmt.Sprintf("%q", elem))
	}
	return quotedElems
}

func StringSetToList(set *schema.Set) []string {
	var list []string
	for _, item := range set.List() {
		list = append(list, item.(string))
	}
	return list
}

func StringListToSet(list []string) *schema.Set {
	var set []interface{}
	for _, item := range list {
		set = append(set, item)
	}
	return schema.NewSet(schema.HashString, set)
}

func FormatQuery(ctx context.Context, query string, meta any) string {
	client := meta.(*ApiClient)
	conn := client.ClickhouseConnection
	formatQueryStmt := `SELECT formatQuery($1)`
	row := (*conn).QueryRow(ctx, formatQueryStmt, query)

	var formattedQueryResult string
	if err := row.Scan(&formattedQueryResult); err != nil {
		return ""
	}
	return formattedQueryResult
}

// NormalizeQuery converts a regular query to lowercase, and strips new lines.
// This ensures that queries stored in the state are consistent with the queries
// stored in Clickhouse's `system.tables`
func NormalizeQuery(query string) string {
	query = strings.ToLower(query)
	query = strings.ReplaceAll(query, "\n", " ")
	query = strings.Join(strings.Fields(query), " ")

	return query
}
