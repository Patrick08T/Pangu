package trace

import (
	"errors"
	"net/url"
	"strconv"
)

type Query struct {
	After, Before int64
	Limit         int
	NodeID        string
	Kind          string
	Status        string
}

func ParseQuery(values url.Values) (Query, error) {
	query := Query{Limit: 50}
	invalid := errors.New("追踪查询无效：游标非负，limit 为 1–100，筛选不可重复")
	for key, items := range values {
		if len(items) != 1 || items[0] == "" {
			return query, invalid
		}
		value := items[0]
		var err error
		switch key {
		case "after":
			query.After, err = strconv.ParseInt(value, 10, 64)
		case "before":
			query.Before, err = strconv.ParseInt(value, 10, 64)
		case "limit":
			query.Limit, err = strconv.Atoi(value)
		case "node":
			query.NodeID = value
			if len(value) > 128 {
				err = invalid
			}
		case "kind":
			query.Kind = value
			if value != "node" && value != "model" && value != "tool" {
				err = invalid
			}
		case "status":
			query.Status = value
			if value != "running" && value != "succeeded" && value != "failed" && value != "cancelled" {
				err = invalid
			}
		default:
			err = invalid
		}
		if err != nil {
			return query, invalid
		}
	}
	if query.After < 0 || query.Before < 0 || query.Limit < 1 || query.Limit > 100 || query.Before > 0 && query.Before <= query.After {
		return query, invalid
	}
	return query, nil
}
