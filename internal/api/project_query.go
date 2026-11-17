package api

import (
	"errors"
	"net/url"
	"strconv"

	"github.com/google/uuid"
)

func projectQuery(values url.Values, required bool) (string, int, error) {
	project, offset := "", 0
	for key, items := range values {
		if len(items) != 1 || items[0] == "" {
			return "", 0, errors.New("查询参数必须为唯一非空值")
		}
		switch key {
		case "project_id":
			if _, err := uuid.Parse(items[0]); err != nil {
				return "", 0, errors.New("project_id 必须为有效 UUID")
			}
			project = items[0]
		case "offset":
			var err error
			offset, err = strconv.Atoi(items[0])
			if err != nil || offset < 0 {
				return "", 0, errors.New("offset 必须为非负整数")
			}
		default:
			return "", 0, errors.New("只接受 project_id 和 offset")
		}
	}
	if required && project == "" {
		return "", 0, errors.New("请选择项目")
	}
	return project, offset, nil
}
