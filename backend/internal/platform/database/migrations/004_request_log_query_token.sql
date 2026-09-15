-- 只为后续访客请求保存加密的 URL token；历史脱敏值不回填或猜测。
ALTER TABLE request_logs ADD COLUMN query_token_cipher text NOT NULL DEFAULT '';
