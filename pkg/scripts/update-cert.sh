#!/bin/bash


function update_secret {
    # read stdin
    echo "Reading certificate from stdin..."
    read -re new_cert
    
    # install deps
    mkdir -p "$HOME/bin"
    curl -sL "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
      -o "$HOME/bin/kubectl"
    chmod +x "$HOME/bin/kubectl"
    export PATH="$HOME/bin:$PATH"

    cr_name=$(kubectl -n ravendb get ravendbcluster -o jsonpath='{.items[0].metadata.name}')

    if [ "$RAVEN_Setup_Mode" = "LetsEncrypt" ]; then
        secret_name=$(kubectl -n ravendb get ravendbcluster "$cr_name" -o "jsonpath={.spec.nodes[?(@.tag=='$NODE_TAG')].certSecretRef}")
    fi

    if [ "$RAVEN_Setup_Mode" = "None" ]; then
        secret_name=$(kubectl -n ravendb get ravendbcluster "$cr_name" -o "jsonpath={.spec.clusterCertSecretRef}")
    fi

    previous_content=$(kubectl get secret "$secret_name" -n ravendb -o jsonpath='{.data.server\.pfx}')
    echo "Previous secret (first 80 chars): ${previous_content:0:80}"

    # update secret
    echo "Updating server certificate on node server by updating ravendb-certs secret"
    kubectl get secret "$secret_name" -o json -n ravendb | \
        jq ".data[\"server.pfx\"]=\"$new_cert\"" | \
        kubectl apply -f -

    content=$(kubectl get secret "$secret_name" -n ravendb -o jsonpath='{.data.server\.pfx}')
    echo "New secret (first 80 chars): ${content:0:80}"

    if [[ $previous_content == "$content" ]]; then
        echo "ERROR: Kubernetes secret content did not change..."
        exit 111
    fi
}

update_secret >> ${HOME}/cert-update.log