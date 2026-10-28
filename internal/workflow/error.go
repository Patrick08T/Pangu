package workflow

import "errors"

var ErrStopUnconfirmed = errors.New("无法确认执行器全部进程已停止")
