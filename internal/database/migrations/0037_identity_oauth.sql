-- 协议切换使尚未完成的登录失效；已登录会话不受影响。
DELETE FROM pangu.login_states;
ALTER TABLE pangu.login_states DROP COLUMN verifier;
