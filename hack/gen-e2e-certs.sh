#!/usr/bin/env bash
#
# Generate a self-contained set of RavenDB e2e certificates: one cluster CA, a
# server certificate per node (A-F) signed by that CA, and an admin client
# certificate. Written to the E2E_*_PATH locations the suite already consumes.
#
# Why generate instead of shipping fixed PFX secrets: the previous *_PFX_B64
# secrets were fixed fixtures with a bounded validity. When they expired the
# whole e2e suite broke on unchanged code (RavenDB refuses to derive its admin
# client cert from an expired server cert: "notBefore later than notAfter").
# Generating fresh, long-lived certs on every run removes that time-bomb, and
# the bootstrapper already trusts the admin client cert at runtime via
# `rvn admin-channel trustClientCert`, so no pre-registration is needed. Nodes
# trust each other through the shared CA embedded in each PFX chain.

set -euo pipefail

: "${E2E_BASE:?E2E_BASE must be set}"
: "${E2E_CLIENT_PFX_PATH:?E2E_CLIENT_PFX_PATH must be set}"
: "${E2E_CA_CERT_PATH:=$E2E_BASE/ca.crt}"

DOMAIN="ravendb-operator-e2e.ravendb.run"
DAYS="${E2E_CERT_DAYS:-3650}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Cluster CA (shared issuer so nodes trust each other and the admin client).
# RavenDB trusts cluster peers whose certs are signed by this shared CA, the
# same model its setup package uses; the bootstrapper's curl trusts it via the
# ravendb-ca-cert secret (caCertSecretRef -> --cacert).
openssl req -x509 -newkey rsa:2048 -keyout "$WORK/ca.key" -out "$WORK/ca.crt" \
  -days "$DAYS" -nodes -subj "/CN=RavenDB E2E Cluster CA" 2>/dev/null
mkdir -p "$(dirname "$E2E_CA_CERT_PATH")"
install -m 0644 "$WORK/ca.crt" "$E2E_CA_CERT_PATH"

# gen_pfx <common-name> <out-pfx-path> [subjectAltName]
gen_pfx() {
  local cn="$1" out="$2" san="${3:-}"
  mkdir -p "$(dirname "$out")"
  openssl req -newkey rsa:2048 -keyout "$WORK/k.key" -out "$WORK/k.csr" \
    -nodes -subj "/CN=${cn}" 2>/dev/null
  {
    echo "extendedKeyUsage=serverAuth,clientAuth"
    echo "keyUsage=digitalSignature,keyEncipherment,keyCertSign"
    [ -n "$san" ] && echo "subjectAltName=${san}"
  } >"$WORK/ext.cnf"
  openssl x509 -req -in "$WORK/k.csr" -CA "$WORK/ca.crt" -CAkey "$WORK/ca.key" \
    -CAcreateserial -out "$WORK/k.crt" -days "$DAYS" -extfile "$WORK/ext.cnf" 2>/dev/null
  # Include the CA in the PFX chain so RavenDB sees the issuer. Force legacy
  # PKCS#12 algorithms (3DES + SHA-1 MAC): the operator decodes the client PFX
  # with golang.org/x/crypto/pkcs12, which only supports legacy encryption and
  # rejects OpenSSL 3.x defaults (AES/SHA-256) with "unknown digest algorithm".
  # RavenDB's .NET loader accepts legacy too, so this works for both.
  openssl pkcs12 -export -out "$out" -inkey "$WORK/k.key" -in "$WORK/k.crt" \
    -certfile "$WORK/ca.crt" \
    -keypbe PBE-SHA1-3DES -certpbe PBE-SHA1-3DES -macalg SHA1 \
    -passout pass: 2>/dev/null
  chmod 644 "$out"
}

# Admin client certificate (registered at bootstrap by init-cluster.sh).
gen_pfx "admin.client.${DOMAIN}" "$E2E_CLIENT_PFX_PATH"

# Per-node server certificates. Directory tag is uppercase (A..F); the hostname
# in publicServerUrl is lowercase (a..f).
for U in A B C D E F; do
  l="$(printf '%s' "$U" | tr '[:upper:]' '[:lower:]')"
  var="E2E_NODE_${U}_PFX_PATH"
  out="${!var:-}"
  [ -z "$out" ] && { echo "warning: ${var} not set, skipping node ${U}" >&2; continue; }
  gen_pfx "${l}.${DOMAIN}" "$out" "DNS:${l}.${DOMAIN},DNS:${l}-tcp.${DOMAIN}"
done

echo "generated e2e certs under ${E2E_BASE} (CA + admin client + nodes A-F, valid ${DAYS}d)"
