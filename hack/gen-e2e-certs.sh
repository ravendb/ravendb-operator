#!/usr/bin/env bash
#
# Generate a self-contained set of RavenDB e2e certificates for Mode=None
# (bring-your-own-cert): one cluster CA, one cluster server certificate (with
# SANs covering every node hostname), and one admin client certificate. Written
# to the E2E_*_PATH locations the suite consumes.
#
# Why generate instead of shipping fixed *_PFX_B64 secrets: the previous secrets
# were fixed Let's Encrypt certs that expired, silently breaking the whole e2e
# suite on unchanged code. Generating fresh, long-lived certs each run removes
# that time-bomb. RavenDB trusts cluster peers via the shared CA (all nodes use
# the same cluster cert here); the admin client cert is trusted at bootstrap
# (rvn admin-channel trustClientCert); the bootstrapper's curl trusts the server
# via the CA (caCertSecretRef -> --cacert). PKCS#12 is emitted with legacy
# algorithms (3DES + SHA-1) because the operator decodes the client cert with
# golang.org/x/crypto/pkcs12, which rejects OpenSSL 3.x AES/SHA-256 defaults.

set -euo pipefail

: "${E2E_BASE:?E2E_BASE must be set}"
: "${E2E_CLUSTER_PFX_PATH:?E2E_CLUSTER_PFX_PATH must be set}"
: "${E2E_CLIENT_PFX_PATH:?E2E_CLIENT_PFX_PATH must be set}"
: "${E2E_CA_CERT_PATH:=$E2E_BASE/ca.crt}"

DOMAIN="ravendb-operator-e2e.ravendb.run"
DAYS="${E2E_CERT_DAYS:-3650}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Cluster CA (shared issuer).
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
  # Legacy PKCS#12 (3DES + SHA-1 MAC) for golang.org/x/crypto/pkcs12 + .NET.
  openssl pkcs12 -export -out "$out" -inkey "$WORK/k.key" -in "$WORK/k.crt" \
    -certfile "$WORK/ca.crt" \
    -keypbe PBE-SHA1-3DES -certpbe PBE-SHA1-3DES -macalg SHA1 \
    -passout pass: 2>/dev/null
  chmod 644 "$out"
}

# Single cluster server certificate, SANs for every node hostname (a-f) + tcp.
san="DNS:${DOMAIN}"
for l in a b c d e f; do
  san="${san},DNS:${l}.${DOMAIN},DNS:${l}-tcp.${DOMAIN}"
done
gen_pfx "${DOMAIN}" "$E2E_CLUSTER_PFX_PATH" "$san"

# Admin client certificate (registered at bootstrap by init-cluster.sh).
gen_pfx "admin.client.${DOMAIN}" "$E2E_CLIENT_PFX_PATH"

echo "generated e2e certs under ${E2E_BASE} (CA + cluster cert + admin client, valid ${DAYS}d)"
