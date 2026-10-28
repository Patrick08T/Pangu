package identity

import (
	"context"
	"net/url"

	"github.com/Patrick08T/Pangu/internal/database"
)

func beginLogin(ctx context.Context, namespace string, cfg Config) (string, string, error) {
	state, err := randomToken()
	if err != nil {
		return "", "", err
	}
	browser, err := randomToken()
	if err != nil {
		return "", "", err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return "", "", err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `INSERT INTO pangu.login_states(state_hash,browser_hash,namespace,app_id,redirect_uri,expires_at) VALUES($1,$2,$3,$4,$5,now()+interval '5 minutes')`, digest(state), digest(browser), namespace, cfg.AppID, cfg.callback())
	if err != nil {
		return "", "", err
	}
	values := url.Values{"client_id": {cfg.AppID}, "response_type": {"code"}, "redirect_uri": {cfg.callback()}, "state": {state}}
	return "https://accounts.feishu.cn/open-apis/authen/v1/authorize?" + values.Encode(), browser, nil
}

func consumeLogin(ctx context.Context, namespace string, cfg Config, state, browser string) error {
	if len(state) != 43 || len(browser) != 43 {
		return ErrUnauthenticated
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	result, err := conn.Exec(ctx, `DELETE FROM pangu.login_states WHERE state_hash=$1 AND browser_hash=$2 AND namespace=$3 AND app_id=$4 AND redirect_uri=$5 AND expires_at>now()`, digest(state), digest(browser), namespace, cfg.AppID, cfg.callback())
	if err == nil && result.RowsAffected() != 1 {
		return ErrUnauthenticated
	}
	return err
}
