package usage

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"

	"github.com/Patrick08T/Pangu/internal/workflow"
)

type failedTransport struct{ err error }

func (f failedTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, f.err }

func TestOnlyModelTransportErrorsAreRetryable(t *testing.T) {
	for _, tt := range []struct {
		kind string
		err  error
		want string
	}{
		{"model", &net.DNSError{IsTimeout: true}, "model_network"},
		{"model", io.ErrUnexpectedEOF, "model_network"},
		{"model", context.Canceled, "cancelled"},
		{"model", errors.New("工具执行失败"), "execution"},
		{"search", &net.DNSError{IsTimeout: true}, "execution"},
	} {
		request, _ := http.NewRequest("POST", "https://model.invalid", nil)
		_, err := (&Transport{Kind: tt.kind, Base: failedTransport{tt.err}}).RoundTrip(request)
		if kind := workflow.FailureKind(err); kind != tt.want {
			t.Fatalf("%s: 分类=%s 期望=%s", tt.kind, kind, tt.want)
		}
	}
}
