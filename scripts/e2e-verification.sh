#!/usr/bin/env bash
set -uo pipefail

BASE="${BASE:-http://localhost:8080}"
ENV_FILE="${ENV_FILE:-$(dirname "$0")/../.env}"
PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); echo "  PASS  $1"; }
bad() { FAIL=$((FAIL+1)); echo "  FAIL  $1"; }

jget() { python3 -c "
import json,sys
d=json.load(sys.stdin)
print($1)" 2>/dev/null; }

code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

INTERNAL=$(grep -E '^INTERNAL_SERVICE_SECRET=' "$ENV_FILE" | cut -d= -f2-)
RECEIPT=$(grep -E '^RECEIPT_SECRET=' "$ENV_FILE" | cut -d= -f2-)
[ -n "$INTERNAL" ] && ok "read INTERNAL_SERVICE_SECRET from env" || bad "INTERNAL_SERVICE_SECRET missing in $ENV_FILE"
[ -n "$RECEIPT" ] && ok "read RECEIPT_SECRET from env" || bad "RECEIPT_SECRET missing in $ENV_FILE"

S=$(date +%s)
mkcitizen() {
  local tag="$1"
  local aadhaar
  aadhaar=$(printf "%012d" $(( (S + tag) % 1000000000000 )))
  curl -s -X POST "$BASE/auth/register" -H 'Content-Type: application/json' \
    -d "{\"fullName\":\"Citizen $tag\",\"aadhaarId\":\"$aadhaar\",\"email\":\"c${tag}${S}@example.com\",\"password\":\"password123\"}" >/dev/null
  curl -s -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
    -d "{\"aadhaarId\":\"$aadhaar\",\"password\":\"password123\"}" > "/tmp/yomi-c$tag.json"
  jget "d['data']['citizen']['id']" < "/tmp/yomi-c$tag.json"
}
token_of() { jget "d['data']['accessToken']" < "/tmp/yomi-c$1.json"; }

echo "== citizens =="
C1=$(mkcitizen 1); T1=$(token_of 1)
C2=$(mkcitizen 2); T2=$(token_of 2)
[ -n "$C1" ] && ok "citizen 1 registered ($C1)" || bad "citizen 1 registration"
[ -n "$C2" ] && ok "citizen 2 registered ($C2)" || bad "citizen 2 registration"

echo "== institutions with role ceilings =="
VIEWER=$(curl -s -X POST "$BASE/institution/register" -H 'Content-Type: application/json' \
  -d "{\"name\":\"Viewer Bank $S\",\"sector\":\"banking\",\"role\":\"viewer\"}")
VKEY=$(echo "$VIEWER" | jget "d['apiKey']")
VID=$(echo "$VIEWER" | jget "d['data']['id']")
[ -n "$VKEY" ] && ok "viewer institution registered (role=$(echo "$VIEWER" | jget "d['data']['role']"))" || bad "viewer institution: $VIEWER"

ANALYST=$(curl -s -X POST "$BASE/institution/register" -H 'Content-Type: application/json' \
  -d "{\"name\":\"Analyst Hospital $S\",\"sector\":\"healthcare\",\"role\":\"analyst\"}")
AKEY=$(echo "$ANALYST" | jget "d['apiKey']")
AID=$(echo "$ANALYST" | jget "d['data']['id']")
[ -n "$AKEY" ] && ok "analyst institution registered (role=$(echo "$ANALYST" | jget "d['data']['role']"))" || bad "analyst institution: $ANALYST"

DEFAULT=$(curl -s -X POST "$BASE/institution/register" -H 'Content-Type: application/json' \
  -d "{\"name\":\"Default Co $S\",\"sector\":\"telecom\"}")
[ "$(echo "$DEFAULT" | jget "d['data']['role']")" = "viewer" ] && ok "role defaults to viewer" || bad "default role: $DEFAULT"

echo "== scope ceiling is enforced =="
EXP=$(( S + 86400 ))
OUT=$(curl -s -X POST "$BASE/consent/request" -H "X-API-Key: $VKEY" -H 'Content-Type: application/json' \
  -d "{\"citizenId\":\"$C1\",\"dataScope\":[\"health.records\"],\"purpose\":\"Checkup\",\"expiresAt\":\"$(date -u -d @$EXP +%Y-%m-%dT%H:%M:%SZ)\"}")
RC=$(code -X POST "$BASE/consent/request" -H "X-API-Key: $VKEY" -H 'Content-Type: application/json' \
  -d "{\"citizenId\":\"$C1\",\"dataScope\":[\"health.records\"],\"purpose\":\"Checkup\",\"expiresAt\":\"$(date -u -d @$EXP +%Y-%m-%dT%H:%M:%SZ)\"}")
[ "$RC" = "403" ] && ok "viewer asking for health.records -> 403 ($(echo "$OUT" | jget "d['code']"))" || bad "viewer health scope: $RC $OUT"

RC=$(code -X POST "$BASE/consent/request" -H "X-API-Key: $VKEY" -H 'Content-Type: application/json' \
  -d "{\"citizenId\":\"$C1\",\"dataScope\":[\"not.a.scope\"],\"purpose\":\"Nope\",\"expiresAt\":\"$(date -u -d @$EXP +%Y-%m-%dT%H:%M:%SZ)\"}")
[ "$RC" = "400" ] && ok "unknown scope key -> 400" || bad "unknown scope: $RC"

echo "== consent lifecycle =="
REQ=$(curl -s -X POST "$BASE/consent/request" -H "X-API-Key: $AKEY" -H 'Content-Type: application/json' \
  -d "{\"citizenId\":\"$C1\",\"dataScope\":[\"telecom.usage\",\"identity.basic\"],\"purpose\":\"Network audit\",\"expiresAt\":\"$(date -u -d @$EXP +%Y-%m-%dT%H:%M:%SZ)\"}")
RID=$(echo "$REQ" | jget "d['data']['id']")
[ -n "$RID" ] && ok "consent request created ($RID)" || bad "consent request: $REQ"
[ "$(echo "$REQ" | jget "'authToken' in d['data']")" = "False" ] && ok "request payload carries no auth token" || bad "request leaked a token field"

PEND=$(curl -s "$BASE/consent/pending/$C1" -H "Authorization: Bearer $T1")
case "$PEND" in *'"data":[]'*|*'"data":['*) ok "pending returns an array" ;; *) bad "pending: $PEND" ;; esac

APP=$(curl -s -X POST "$BASE/consent/approve/$RID" -H "Authorization: Bearer $T1")
ATOKEN=$(echo "$APP" | jget "d['data']['accessToken']")
STHASH=$(echo "$APP" | jget "d['data']['request']['accessTokenHash']")
[ -n "$ATOKEN" ] && ok "approval issued an access token (${#ATOKEN} chars)" || bad "approve: $APP"
[ "$(echo "$APP" | jget "'authToken' in d['data']['request']")" = "False" ] && ok "stored record has no authToken field" || bad "approve leaked authToken"
[ "$STHASH" != "$ATOKEN" ] && ok "ledger stores a hash, not the token" || bad "stored value equals the token"
case "$ATOKEN" in yomi_access_*) ok "token has the yomi_access_ prefix" ;; *) bad "unexpected token shape: $ATOKEN" ;; esac
case "$APP" in *"\"signature\""*) bad "approve response carries an HMAC signature" ;; *) ok "no HMAC signature reaches the client" ;; esac

echo "== institution can check a token it holds =="
V=$(curl -s -X POST "$BASE/consent/verify-token" -H "X-Yomi-Internal: $INTERNAL" -H 'Content-Type: application/json' \
  -d "{\"accessToken\":\"$ATOKEN\"}")
[ "$(echo "$V" | jget "d['data']['active']")" = "True" ] && ok "verify-token says active" || bad "verify-token: $V"

RC=$(code -X POST "$BASE/consent/verify-token" -H 'Content-Type: application/json' -d "{\"accessToken\":\"$ATOKEN\"}")
[ "$RC" = "403" ] && ok "verify-token without the internal secret -> 403" || bad "unauthenticated verify-token: $RC"

echo "== citizen receipt =="
EID=""
for _ in 1 2 3 4 5 6; do
  TRAIL=$(curl -s "$BASE/audit/citizen/$C1" -H "Authorization: Bearer $T1")
  EID=$(echo "$TRAIL" | jget "[e['id'] for e in d['data'] if e['eventType']=='consent_approved'][0]")
  [ -n "$EID" ] && break
  sleep 2
done
[ -n "$EID" ] && ok "approval event is in the ledger ($EID)" || bad "no consent_approved entry: $TRAIL"

REC=$(curl -s "$BASE/audit/receipt/$EID" -H "Authorization: Bearer $T1")
echo "$REC" | python3 -c "import json,sys; json.dump(json.load(sys.stdin)['data'], sys.stdout)" > /tmp/yomi-receipt.json
SIG=$(jget "d['signature']" < /tmp/yomi-receipt.json)
CONTENT=$(jget "d['content']" < /tmp/yomi-receipt.json)
[ -n "$SIG" ] && ok "receipt carries a signature" || bad "receipt: $REC"

EXPECTED=$(python3 -c "
import hmac,hashlib,json,sys
r=json.load(open('/tmp/yomi-receipt.json'))
print(hmac.new(sys.argv[1].encode(), r['content'].encode(), hashlib.sha256).hexdigest())" "$RECEIPT")
[ "$EXPECTED" = "$SIG" ] && ok "signature reproduces offline from the canonical string" || bad "offline HMAC mismatch: $EXPECTED vs $SIG"

case "$CONTENT" in *"$ATOKEN"*) bad "the receipt content leaks the raw access token" ;; *) ok "receipt content carries no raw token" ;; esac

VR=$(curl -s -X POST "$BASE/audit/receipt/verify" -H 'Content-Type: application/json' -d @/tmp/yomi-receipt.json)
[ "$(echo "$VR" | jget "d['data']['valid']")" = "True" ] && ok "receipt verifies against the ledger" || bad "receipt verify: $VR"

echo "== an edited receipt cannot lie about the ledger =="
python3 - <<'PY'
import json
r = json.load(open('/tmp/yomi-receipt.json'))
r['eventType'] = 'consent_revoked'
r['metadata'] = {'scope': ['everything'], 'institution_role': 'admin'}
json.dump(r, open('/tmp/yomi-receipt-tampered.json','w'))
PY
TR=$(curl -s -X POST "$BASE/audit/receipt/verify" -H 'Content-Type: application/json' -d @/tmp/yomi-receipt-tampered.json)
[ "$(echo "$TR" | jget "d['data']['valid']")" = "True" ] && ok "ledger answers for itself, not for the file" || bad "verify failed: $TR"
[ "$(echo "$TR" | jget "d['data']['receipt']['eventType']")" = "consent_approved" ] && ok "tampered event type is overridden by the ledger" || bad "ledger echoed the tampered event type: $TR"
python3 - <<'PY' > /tmp/yomi-diff.txt
import json
presented = json.load(open('/tmp/yomi-receipt-tampered.json'))
ledger = json.load(open('/tmp/yomi-receipt.json'))
print('different' if presented != ledger else 'identical')
PY
[ "$(cat /tmp/yomi-diff.txt)" = "different" ] && ok "an edited file is detectable by diffing against the returned receipt" || bad "diff produced no difference"

python3 - <<'PY'
import json
r = json.load(open('/tmp/yomi-receipt.json'))
r['receiptId'] = '00000000-0000-4000-8000-000000000000'
json.dump(r, open('/tmp/yomi-receipt-ghost.json','w'))
PY
GR=$(curl -s -X POST "$BASE/audit/receipt/verify" -H 'Content-Type: application/json' -d @/tmp/yomi-receipt-ghost.json)
[ "$(echo "$GR" | jget "d['data']['reason']")" = "NOT_ISSUED" ] && ok "unknown receipt id -> NOT_ISSUED" || bad "ghost receipt: $GR"

python3 - <<'PY'
import json
r = json.load(open('/tmp/yomi-receipt.json'))
r['signature'] = 'a' * 64
json.dump(r, open('/tmp/yomi-receipt-forged.json','w'))
PY
FR=$(curl -s -X POST "$BASE/audit/receipt/verify" -H 'Content-Type: application/json' -d @/tmp/yomi-receipt-forged.json)
[ "$(echo "$FR" | jget "d['data']['reason']")" = "SIGNATURE_MISMATCH" ] && ok "forged signature -> SIGNATURE_MISMATCH" || bad "forged receipt: $FR"

echo "== receipt verify needs no account =="
RC=$(code -X POST "$BASE/audit/receipt/verify" -H 'Content-Type: application/json' -d @/tmp/yomi-receipt.json)
[ "$RC" = "200" ] && ok "public verify with no token -> 200" || bad "public verify: $RC"
RC=$(code "$BASE/audit/receipt/$EID")
[ "$RC" = "401" ] && ok "minting a receipt still needs a token -> 401" || bad "unauthenticated receipt mint: $RC"

echo "== a citizen can check a token they hold =="
CT=$(curl -s -X POST "$BASE/consent/check-token" -H "Authorization: Bearer $T1" -H 'Content-Type: application/json' \
  -d "{\"accessToken\":\"$ATOKEN\"}")
[ "$(echo "$CT" | jget "d['data']['active']")" = "True" ] && ok "citizen check-token says active" || bad "citizen check-token: $CT"
RC=$(code -X POST "$BASE/consent/check-token" -H 'Content-Type: application/json' -d "{\"accessToken\":\"$ATOKEN\"}")
[ "$RC" = "401" ] && ok "check-token without a citizen session -> 401" || bad "unauthenticated check-token: $RC"

echo "== revocation takes effect immediately =="
curl -s -X DELETE "$BASE/consent/revoke/$RID" -H "Authorization: Bearer $T1" >/dev/null
V=$(curl -s -X POST "$BASE/consent/verify-token" -H "X-Yomi-Internal: $INTERNAL" -H 'Content-Type: application/json' \
  -d "{\"accessToken\":\"$ATOKEN\"}")
[ "$(echo "$V" | jget "d['data']['active']")" = "False" ] && ok "revoked token is inactive ($(echo "$V" | jget "d['data']['reason']"))" || bad "revoked token still active: $V"

CT=$(curl -s -X POST "$BASE/consent/check-token" -H "Authorization: Bearer $T1" -H 'Content-Type: application/json' \
  -d "{\"accessToken\":\"$ATOKEN\"}")
[ "$(echo "$CT" | jget "d['data']['reason']")" = "REVOKED" ] && ok "citizen check-token reports REVOKED" || bad "citizen check-token: $CT"

VR=$(curl -s -X POST "$BASE/audit/receipt/verify" -H 'Content-Type: application/json' -d @/tmp/yomi-receipt.json)
[ "$(echo "$VR" | jget "d['data']['valid']")" = "True" ] && ok "receipt still verifies after revocation" || bad "receipt broke on revocation: $VR"

echo "== a citizen cannot read another citizen's ledger =="
RC=$(code "$BASE/audit/citizen/$C1" -H "Authorization: Bearer $T2")
[ "$RC" = "403" ] && ok "cross-citizen ledger read -> 403" || bad "cross-citizen ledger read: $RC"
RC=$(code "$BASE/audit/event/$EID" -H "Authorization: Bearer $T2")
[ "$RC" = "403" ] && ok "cross-citizen entry read -> 403" || bad "cross-citizen entry read: $RC"
RC=$(code "$BASE/audit/receipt/$EID" -H "Authorization: Bearer $T2")
[ "$RC" = "403" ] && ok "cross-citizen receipt read -> 403" || bad "cross-citizen receipt read: $RC"

echo "== chain integrity =="
CH=$(curl -s "$BASE/audit/verify" -H "Authorization: Bearer $T1")
[ "$(echo "$CH" | jget "d['data']['intact']")" = "True" ] && ok "hash chain intact over $(echo "$CH" | jget "d['data']['entriesChecked']") entries" || bad "chain: $CH"

echo
echo "RESULT: $PASS passed, $FAIL failed"
[ "$FAIL" = "0" ]