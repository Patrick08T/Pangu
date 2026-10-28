package workflow

import "errors"

type Budget struct {
	ModelCalls    int `json:"model_calls"`
	SearchCalls   int `json:"search_calls"`
	CodexSessions int `json:"codex_sessions"`
}

func (b *Budget) Validate() error {
	if b == nil {
		return nil
	}
	for _, value := range []int{b.ModelCalls, b.SearchCalls, b.CodexSessions} {
		if value < 0 || value > 4096 {
			return errors.New("各类调用预算必须在 0 至 4096 之间")
		}
	}
	return nil
}

type Price struct {
	Currency               string `json:"currency"`
	InputPerMillionMicros  int64  `json:"input_per_million_micros"`
	OutputPerMillionMicros int64  `json:"output_per_million_micros"`
}

func (p *Price) Validate() error {
	if p == nil {
		return nil
	}
	if p.Currency != "USD" && p.Currency != "CNY" {
		return errors.New("估算价格币种仅支持 USD 或 CNY")
	}
	for _, v := range []int64{p.InputPerMillionMicros, p.OutputPerMillionMicros} {
		if v < 0 || v > 1000000000 {
			return errors.New("每百万 token 的估算单价必须为 0 至 1000000000 微货币单位")
		}
	}
	return nil
}
