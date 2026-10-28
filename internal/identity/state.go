package identity

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"

	"github.com/Patrick08T/Pangu/internal/database"
	"github.com/jackc/pgx/v5"
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
	verifier, err := randomToken()
	if err != nil {
		return "", "", err
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return "", "", err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `INSERT INTO pangu.login_states(state_hash,browser_hash,namespace,app_id,redirect_uri,verifier,expires_at) VALUES($1,$2,$3,$4,$5,$6,now()+interval '5 minutes')`, digest(state), digest(browser), namespace, cfg.AppID, cfg.callback(), verifier)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	values := url.Values{"client_id": {cfg.AppID}, "response_type": {"code"}, "redirect_uri": {cfg.callback()}, "state": {state}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}}
	return "https://accounts.feishu.cn/open-apis/authen/v1/authorize?" + values.Encode(), browser, nil
}

func consumeLogin(ctx context.Context, namespace string, cfg Config, state, browser string) (string, error) {
	if len(state) != 43 || len(browser) != 43 {
		return "", ErrUnauthenticated
	}
	conn, err := database.Open(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close(context.Background())
	var verifier string
	err = conn.QueryRow(ctx, `DELETE FROM pangu.login_states WHERE state_hash=$1 AND browser_hash=$2 AND namespace=$3 AND app_id=$4 AND redirect_uri=$5 AND expires_at>now() RETURNING verifier`, digest(state), digest(browser), namespace, cfg.AppID, cfg.callback()).Scan(&verifier)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnauthenticated
	}
	return verifier, err
}
