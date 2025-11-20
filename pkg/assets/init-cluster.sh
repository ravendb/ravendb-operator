#!/bin/bash

set -euo pipefail

function log() {
    echo "[$(date '+%H:%M:%S')] $1"
}

function convert_pfx_to_pem_and_key() {
    local pfx=$1
    local cert_out=$2
    local key_out=$3

    log "Converting $pfx to PEM and KEY..."
    openssl pkcs12 -legacy -in "$pfx" -clcerts -nokeys -out "$cert_out" -passin pass:
    openssl pkcs12 -legacy -in "$pfx" -nocerts -nodes -out "$key_out" -passin pass:
}

function install_deps() {
    mkdir -p "$HOME/bin"
    curl -sL "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
      -o "$HOME/bin/kubectl"
    chmod +x "$HOME/bin/kubectl"
    export PATH="$HOME/bin:$PATH"
}


function register_admin_cert() {
    log "Registering Admin client certificate..."
    local pfx_src="$CLIENT_PFX"
    local first_tag="${TAGS%% *}"
    local first_pod="ravendb-$(echo "$first_tag" | tr '[:upper:]' '[:lower:]')-0"
    local ns="ravendb"

    kubectl -n "$ns" exec -i "$first_pod" -- sh -c 'cat > /tmp/client.pfx && chmod 0644 /tmp/client.pfx' < "$pfx_src"

  log "Registering client cert via rvn on first node..."
    kubectl -n "$ns" exec -i "$first_pod" -- /usr/lib/ravendb/server/rvn admin-channel \
    <<< 'trustClientCert client /tmp/client.pfx' >/dev/null 2>&1

  log "Client cert registered on the first node."
}

function join_node_to_cluster() {
    local tag=$1
    local url=$2
    local is_watcher=$3

    tag=${tag^^}

    local curl_args=(
        -s -S -o /dev/null -w "%{http_code}"
        --cert "$CLIENT_CERT_PEM"
        --key "$CLIENT_KEY_PEM"
        "${CURL_CA_ARGS[@]}"
        -X PUT
        -G "$LEADER_URL/admin/cluster/node"
        --data-urlencode "url=$url"
        --data-urlencode "tag=$tag"
    )

    if [[ "$is_watcher" == "true" ]]; then  # left here for future use 
        curl_args+=( --data-urlencode "watcher=true" )
    fi

      local response
    response=$(curl "${curl_args[@]}" -w "\n%{http_code}")

    local http_code
    http_code=$(echo "$response" | tail -n1)

    if [[ "$http_code" =~ ^20[0-9]$ ]]; then
        log "[$tag] added as $( [[ "$is_watcher" == "true" ]] && echo Watcher || echo Member )"
    else
        log "Failed to add [$tag] to cluster. HTTP $http_code"
        echo "$response" | head -n -1
        exit 1
    fi
}

function print_topology() {
    log "Cluster topology:"
    curl -s --cert "$CLIENT_CERT_PEM" --key "$CLIENT_KEY_PEM" "${CURL_CA_ARGS[@]}" \
        "$LEADER_URL/cluster/topology" | jq '{
        Leader,
        CurrentState,
        CurrentTerm,
        TopologyId: .Topology.TopologyId,
        Members: (.Topology.Members | keys | join(" ")),
    }'
    echo
}

function tag_from_url() {
    local url="$1"
    printf "%s" "$(echo "$url" | sed -E 's#^https?://([A-Za-z]).*#\1#')"
}

log "=== Starting Cluster Initialization ==="

SERVER_PFX="/ravendb/certs/server.pfx"
CLIENT_PFX="/ravendb/client-certs/client.pfx"
CA_CERT_PATH="/ravendb/ca-cert/ca.crt"
SERVER_CERT_PEM="/tmp/server.pem"
SERVER_KEY_PEM="/tmp/server.key"
CLIENT_CERT_PEM="/tmp/client.pem"
CLIENT_KEY_PEM="/tmp/client.key"

CURL_CA_ARGS=()
if [[ -f "$CA_CERT_PATH" ]]; then
    CURL_CA_ARGS=( --cacert "$CA_CERT_PATH" )
fi

convert_pfx_to_pem_and_key "$SERVER_PFX" "$SERVER_CERT_PEM" "$SERVER_KEY_PEM"
convert_pfx_to_pem_and_key "$CLIENT_PFX" "$CLIENT_CERT_PEM" "$CLIENT_KEY_PEM"

install_deps

register_admin_cert

IFS=' ' read -r -a member_urls <<< "$MEMBER_URLS"

for url in "${member_urls[@]}"; do
    tag="$(tag_from_url "$url")"
    join_node_to_cluster "$tag" "$url" false
    sleep 3
done

print_topology
echo
log "=== Cluster Initialization Complete ==="