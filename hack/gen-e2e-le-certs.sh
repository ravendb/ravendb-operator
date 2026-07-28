#!/usr/bin/env bash
#
# Generate REAL Let's Encrypt certificates for the e2e suite using RavenDB's own
# `rvn create-setup-package`. RavenDB performs the ACME DNS-01 challenge for the
# *.ravendb.run domain via the license, so no DNS-provider credentials are
# needed. Output is base64 blobs for the existing *_PFX_B64 secrets, so nothing
# in the operator, chart, or e2e code changes: this just refreshes the certs.
#
# Runs on the host (needs docker, jq, unzip, base64). Only the rvn call is
# containerized, because rvn ships inside the RavenDB image.
#
# Env:
#   RAVEN_LICENSE   license JSON (the e2e RAVEN_LICENSE secret)     [required]
#   E2E_EMAIL       ACME registration email                         [required]
#   E2E_DOMAIN      subdomain to claim (default: ravendb-operator-e2e)
#   E2E_ROOT_DOMAIN RavenDB-managed root (default: ravendb.run)
#   E2E_TAGS        node tags (default: "a b c d e f")
#   E2E_IMAGE       RavenDB image providing rvn (default: 6.2.11-ubuntu.22.04-x64)
#
# Usage: RAVEN_LICENSE=... E2E_EMAIL=you@example.com hack/gen-e2e-le-certs.sh <out-dir>
set -euo pipefail

OUT="${1:?usage: gen-e2e-le-certs.sh <out-dir>}"
EMAIL="${E2E_EMAIL:?E2E_EMAIL is required}"
DOMAIN="${E2E_DOMAIN:-ravendb-operator-e2e}"
ROOT="${E2E_ROOT_DOMAIN:-ravendb.run}"
TAGS="${E2E_TAGS:-a b c d e f}"
IMAGE="${E2E_IMAGE:-ravendb/ravendb:6.2.11-ubuntu.22.04-x64}"
: "${RAVEN_LICENSE:?RAVEN_LICENSE is required}"

mkdir -p "$OUT"
chmod 777 "$OUT" # the container writes package.zip here as uid 999
base="$DOMAIN.$ROOT"

nodes=$(for t in $TAGS; do
  u=$(printf '%s' "$t" | tr '[:lower:]' '[:upper:]')
  printf '{"%s":{"PublicServerUrl":"https://%s.%s","PublicTcpServerUrl":"tcp://%s.%s:38888","Port":443,"TcpPort":38888,"Addresses":["0.0.0.0"]}}\n' "$u" "$t" "$base" "$t" "$base"
done | jq -s 'add')

jq -n \
  --argjson license "$RAVEN_LICENSE" \
  --arg email "$EMAIL" \
  --arg domain "$DOMAIN" \
  --arg root "$ROOT" \
  --argjson nodes "$nodes" \
  '{License:$license, Email:$email, Domain:$domain, RootDomain:$root, NodeSetupInfos:$nodes}' \
  > "$OUT/setup.json"

echo ">> requesting Let's Encrypt setup package for $base (nodes: $TAGS)"
docker run --rm -v "$OUT:/out" --entrypoint /usr/lib/ravendb/server/rvn "$IMAGE" \
  create-setup-package -m lets-encrypt -s /out/setup.json -o /out/package.zip

unzip -o -q "$OUT/package.zip" -d "$OUT/package"

emit() { # <secret> <pfx>
  base64 -w0 "$2" > "$OUT/$1.txt"
  echo ">> $1 <- ${2#"$OUT/package/"}"
}
emit ADMIN_PFX_B64 "$(find "$OUT/package" -iname '*.client.certificate.*.pfx' | head -1)"
for t in $TAGS; do
  u=$(printf '%s' "$t" | tr '[:lower:]' '[:upper:]')
  emit "${u}_PFX_B64" "$(find "$OUT/package/$u" -iname '*.server.certificate.*.pfx' | head -1)"
done

rm -f "$OUT/setup.json" # contains the license
echo ">> done: base64 blobs in $OUT/*_PFX_B64.txt (set with gh secret set, then delete)"
