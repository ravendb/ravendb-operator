#!/bin/bash

set -euo pipefail

COMMON_CURL_ARGS=(
  -H "User-Agent: ravendb-operator/init-cluster"
)

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
    local ns="${POD_NAMESPACE:?POD_NAMESPACE is required}"

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
        -s -S -w "\n%{http_code}"
        --cert "$CLIENT_CERT_PEM"
        --key "$CLIENT_KEY_PEM"
        "${CURL_CA_ARGS[@]}"
        "${COMMON_CURL_ARGS[@]}"
        -X PUT
        -G "$LEADER_URL/admin/cluster/node"
        --data-urlencode "url=$url"
        --data-urlencode "tag=$tag"
    )

    if [[ "$is_watcher" == "true" ]]; then  # left here for future use 
        curl_args+=( --data-urlencode "watcher=true" )
    fi

    local response
    local curl_rc
    local http_code
    local failure
    local retryable
    local attempt
    local max_attempts=15

    for ((attempt = 1; attempt <= max_attempts; attempt++)); do
        # curl reports transport errors through its exit code and HTTP errors
        # through the status code, so both have to be inspected. Capture the
        # exit code here so `set -e` cannot abort before the retry decision.
        if response=$(curl "${curl_args[@]}"); then
            curl_rc=0
        else
            curl_rc=$?
        fi

        retryable=false

        if [[ "$curl_rc" -eq 0 ]]; then
            http_code=$(echo "$response" | tail -n1)

            if [[ "$http_code" =~ ^20[0-9]$ ]]; then
                log "[$tag] added as $( [[ "$is_watcher" == "true" ]] && echo Watcher || echo Member )"
                return
            fi

            failure="HTTP $http_code"
            # RavenDB briefly answers 307 while the freshly elected leader is
            # not accepting membership changes yet. Following the redirect
            # could turn a Studio page into a false success, so retry instead.
            if [[ "$http_code" == "307" ]]; then
                retryable=true
            fi
        else
            failure="curl exit $curl_rc"
            # The same leader transition can drop the connection before any
            # status code comes back. Retry those transport errors and keep
            # failing fast on setup problems such as a rejected client cert.
            case "$curl_rc" in
                6 | 7 | 28 | 35 | 52 | 55 | 56) retryable=true ;;
            esac
        fi

        if [[ "$retryable" != "true" || "$attempt" -eq "$max_attempts" ]]; then
            log "Failed to add [$tag] to cluster. $failure"
            if [[ "$curl_rc" -eq 0 ]]; then
                echo "$response" | head -n -1
            fi
            exit 1
        fi

        log "[$tag] membership endpoint not ready ($failure); retrying in 2s ($attempt/$max_attempts)"
        sleep 2
    done
}

function bootstrap_leader_with_tag() {
    local first_tag="${TAGS%% *}"
    first_tag=${first_tag^^}

    log "Bootstrapping leader with tag [$first_tag]..."

    local http_code
    http_code=$(curl -s -S -o /dev/null -w "%{http_code}" \
        --cert "$CLIENT_CERT_PEM" \
        --key "$CLIENT_KEY_PEM" \
        "${CURL_CA_ARGS[@]}" \
        "${COMMON_CURL_ARGS[@]}" \
        -X POST \
        -G "$LEADER_URL/admin/cluster/bootstrap" \
        --data-urlencode "tag=$first_tag")

    if [[ ! "$http_code" =~ ^20[0-9]$ ]]; then
        log "Failed to bootstrap leader with tag [$first_tag]. HTTP $http_code"
        exit 1
    fi

    log "Leader bootstrapped with tag [$first_tag]."
}

function print_topology() {
    log "Cluster topology:"
    curl -s "${COMMON_CURL_ARGS[@]}" --cert "$CLIENT_CERT_PEM" --key "$CLIENT_KEY_PEM" "${CURL_CA_ARGS[@]}" \
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

bootstrap_leader_with_tag

IFS=' ' read -r -a member_urls <<< "$MEMBER_URLS"

for url in "${member_urls[@]}"; do
    tag="$(tag_from_url "$url")"
    join_node_to_cluster "$tag" "$url" false
    sleep 3
done

print_topology
echo
log "=== Cluster Initialization Complete ==="
