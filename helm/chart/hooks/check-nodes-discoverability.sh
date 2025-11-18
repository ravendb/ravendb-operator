#!/bin/bash

set -euo pipefail

function log() {
    echo "[$(date '+%H:%M:%S')] $1"
}

function install_kubectl() {
    log "Checking if kubectl is already installed..."
    mkdir -p ~/bin
    export PATH="$HOME/bin:$PATH"

    log "Downloading and installing kubectl..."
    curl -sL -o ~/bin/kubectl "https://dl.k8s.io/release/$(curl -sL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    chmod +x ~/bin/kubectl
    log "kubectl installed successfully."

}

function wait_for_ravendb_pods() {
    log "Waiting for all RavenDB pods to be in 'Running' state..."
    MAX_RETRIES=30

    for ((i=1; i<=MAX_RETRIES; i++)); do
        log "Pod readiness check: attempt $i/$MAX_RETRIES"

        not_ready=$(kubectl get pods -n ravendb -l app.kubernetes.io/name=ravendb \
            -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.phase}{"\n"}{end}' \
            | grep -v '^.* Running$' || true)

        if [[ -z "$not_ready" ]]; then
            log "All RavenDB pods are running."
            return 0
        fi

        log "Waiting for these pods to be ready:"
        log "$not_ready"
        sleep 5
    done

    log "ERROR: Timeout reached. Some RavenDB pods are still not running:"
    log "$not_ready"
    exit 1
}


function check_https_reachability() {
    log "Checking HTTPS (Studio) reachability of RavenDB nodes..."

    IFS=' ' read -r -a URLS_ARR <<< "${URLS}"
    IFS=' ' read -r -a TAGS_ARR <<< "${TAGS}"

    for ((i=0; i<${#URLS_ARR[@]}; i++)); do
        url="${URLS_ARR[$i]}"
        tag="${TAGS_ARR[$i]}"
        log "[$tag] curl -k $url"
        location_header=$(curl -ks -D - "$url" -o /dev/null | grep -i "^location:")
        if echo "$location_header" | grep -q "/studio/index.html"; then
            log "[${tag}] Studio redirect detected - looks good"
        else
            log "[${tag}] Unexpected response from $url"
        fi
    done
}

log "=== Starting Discoverability Checks ==="
install_kubectl
wait_for_ravendb_pods
check_https_reachability
log "=== Discoverability Checks Completed ==="
