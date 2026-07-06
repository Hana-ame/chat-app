#!/usr/bin/env bash
set -uo pipefail

URL='https://upload.moonchan.xyz/api/upload'

pass=0
fail=0

ok()   { pass=$((pass+1)); echo "  ✓ $1"; }
nok()  { fail=$((fail+1)); echo "  ✗ $1"; }

cleanup() {
  rm -f /tmp/upload_test_*.txt /tmp/upload_test_*.png
}
trap cleanup EXIT

extract_id() {
  echo "$1" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4
}

# --------------------------------------------------
echo "=== Upload Service Tests ==="

# 1. Upload a text file (should succeed)
echo "hello world" > /tmp/upload_test_hello.txt
res=$(curl -s -w '\n%{http_code}' -X PUT -F "file=@/tmp/upload_test_hello.txt" "$URL")
body=$(head -n -1 <<< "$res")
code=$(tail -n1 <<< "$res")
if [[ "$code" =~ ^2 ]]; then
  id=$(extract_id "$body")
  if [[ -n "$id" ]]; then
    ok "upload text: HTTP $code, id=$id"
  else
    nok "upload text: HTTP $code but no id in response"
    echo "    body: $body"
  fi
else
  nok "upload text: HTTP $code (expected 2xx)"
  echo "    body: $body"
fi

# 2. Upload a PNG image (generate a 1x1 red pixel)
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82' > /tmp/upload_test_red.png
res=$(curl -s -w '\n%{http_code}' -X PUT -F "file=@/tmp/upload_test_red.png" "$URL")
body=$(head -n -1 <<< "$res")
code=$(tail -n1 <<< "$res")
if [[ "$code" =~ ^2 ]]; then
  id=$(extract_id "$body")
  if [[ -n "$id" ]]; then
    ok "upload png: HTTP $code, id=$id"
  else
    nok "upload png: HTTP $code but no id"
    echo "    body: $body"
  fi
else
  nok "upload png: HTTP $code (expected 2xx)"
  echo "    body: $body"
fi

# 3. Test method OPTIONS (CORS preflight)
code=$(curl -s -o /dev/null -w '%{http_code}' -X OPTIONS -H "Origin: https://chat-app-fastapi.pages.dev" "$URL")
if [[ "$code" -eq 204 ]] || [[ "$code" -eq 200 ]]; then
  ok "OPTIONS: HTTP $code (CORS ok)"
else
  nok "OPTIONS: HTTP $code (expected 204 or 200)"
fi

# --------------------------------------------------
echo "---"
echo "Pass: $pass, Fail: $fail"
if [[ "$fail" -gt 0 ]]; then
  exit 1
fi
