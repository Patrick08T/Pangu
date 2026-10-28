package api

import (
	"errors"
	"net/url"
	"strconv"
)

func catalogQuery(values url.Values, versions bool) (int, error) {
	offset := 0
	for key, items := range values {
		if len(items) != 1 || items[0] == "" {
			return 0, errors.New("查询参数必须为唯一非空值")
		}
		if versions && key != "offset" {
			return 0, errors.New("版本列表只接受 offset")
		}
		switch key {
		case "offset":
			var err error
			offset, err = strconv.Atoi(items[0])
			if err != nil || offset < 0 {
				return 0, errors.New("offset 必须为非负整数")
			}
		case "kind":
			if items[0] != "agent" && items[0] != "workflow" && items[0] != "connection" {
				return 0, errors.New("kind 无效")
			}
		default:
			return 0, errors.New("未知查询参数")
		}
	}
	return offset, nil
}
