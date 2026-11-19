package usage

import "testing"

func TestModelTokensFromBothAPIProtocols(t *testing.T) {
	for _, raw := range []string{
		`{"usage":{"prompt_tokens":12,"completion_tokens":8,"prompt_tokens_details":{"cached_tokens":4}}}`,
		`{"usage":{"input_tokens":12,"output_tokens":8,"input_tokens_details":{"cached_tokens":4}}}`,
	} {
		got := modelTokens([]byte(raw))
		if got == nil || *got != (Tokens{Input: 12, Output: 8, Cached: 4}) {
			t.Errorf("用量解析错误: %+v", got)
		}
	}
}
