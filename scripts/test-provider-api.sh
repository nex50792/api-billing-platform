#!/bin/bash
# Provider向けAPI テスト用curlコマンド集
# 使い方: bash scripts/test-provider-api.sh
#
# 前提: APIサーバーが localhost:8080 で起動していること
#   docker compose up -d db api
#   または go run ./cmd/server

BASE_URL="http://localhost:8080"
COOKIE_FILE="/tmp/provider_cookie.txt"

echo "============================================"
echo " Provider API テスト"
echo "============================================"

# ---- 1. Provider ログイン ----
echo ""
echo "--- 1. Provider ログイン ---"
echo "POST ${BASE_URL}/provider/login"

curl -s -c "$COOKIE_FILE" -w "\nHTTP Status: %{http_code}\n" \
  -X POST "${BASE_URL}/provider/login" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "t3sut0pswd"
  }'

echo ""

# ---- 2. Provider 情報取得 (ログイン確認) ----
echo "--- 2. Provider 自身の情報取得 ---"
echo "GET ${BASE_URL}/provider/me"

curl -s -b "$COOKIE_FILE" -w "\nHTTP Status: %{http_code}\n" \
  "${BASE_URL}/provider/me"

echo ""

# ---- 3. Provider の API 一覧取得 ----
echo "--- 3. Provider の API 一覧取得 ---"
echo "GET ${BASE_URL}/provider/apis"

curl -s -b "$COOKIE_FILE" -w "\nHTTP Status: %{http_code}\n" \
  "${BASE_URL}/provider/apis"

echo ""

# ---- 4. 公開API一覧取得 (認証不要) ----
echo "--- 4. 公開API一覧取得 (認証不要) ---"
echo "GET ${BASE_URL}/provided-apis"

curl -s -b "$COOKIE_FILE" -w "\nHTTP Status: %{http_code}\n" \
  "${BASE_URL}/provided-apis"

echo ""

# ---- 5. API 作成 ----
# echo "--- 5. 新しいAPI作成 ---"
# echo "POST ${BASE_URL}/provided-apis"

# curl -s -b "$COOKIE_FILE" -w "\nHTTP Status: %{http_code}\n" \
#   -X POST "${BASE_URL}/provided-apis" \
#   -H "Content-Type: application/json" \
#   -d '{
#     "name": "Test API",
#     "description": "テスト用API",
#     "price_per_unit": 10.5,
#     "currency": "JPY"
#   }'

# echo ""

# ---- 6. APIキー検証 ----
echo "--- 6. APIキー検証 ---"
echo "POST ${BASE_URL}/provider/keys/verify"
echo "(provided_api_id と api_key は実際の値に置き換えてください)"

curl -s -b "$COOKIE_FILE" -w "\nHTTP Status: %{http_code}\n" \
  -X POST "${BASE_URL}/provider/keys/verify" \
  -H "Content-Type: application/json" \
  -d '{
    "api_key": "fTbfCyk_S2Ls3QeefBR_XMmPyO31y4wkhJ5KGD4Vg3Q",
    "provided_api_id": "4f29f777-ae16-475a-ade1-20e618e921e8"
  }'

echo ""

# ---- 7. Provider ログアウト ----
echo "--- 7. Provider ログアウト ---"
echo "POST ${BASE_URL}/provider/logout"

curl -s -b "$COOKIE_FILE" -c "$COOKIE_FILE" -w "\nHTTP Status: %{http_code}\n" \
  -X POST "${BASE_URL}/provider/logout"

# echo ""
# echo "============================================"
# echo " 個別コマンド (コピペ用)"
# echo "============================================"
# cat << 'EOF'

# # ログイン (Cookieを保存)
# curl -c /tmp/provider_cookie.txt \
#   -X POST http://localhost:8080/provider/login \
#   -H "Content-Type: application/json" \
#   -d '{"email":"test@example.com","password":"your-password"}'

# # APIキー検証
# curl -b /tmp/provider_cookie.txt \
#   -X POST http://localhost:8080/provider/keys/verify \
#   -H "Content-Type: application/json" \
#   -d '{"api_key":"CUSTOMER_KEY","provided_api_id":"API_ID"}'

# # API作成
# curl -b /tmp/provider_cookie.txt \
#   -X POST http://localhost:8080/provided-apis \
#   -H "Content-Type: application/json" \
#   -d '{"name":"My API","description":"説明","price_per_unit":100,"currency":"JPY"}'

# # API一覧 (自分のもの)
# curl -b /tmp/provider_cookie.txt http://localhost:8080/provider/apis

# # ログアウト
# curl -b /tmp/provider_cookie.txt -X POST http://localhost:8080/provider/logout

# EOF

rm -f "$COOKIE_FILE"
