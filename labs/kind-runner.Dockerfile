FROM docker:29-cli@sha256:be132a9f282288de4afaf63379dff75711fda0147c6b72a9df44e51841402144 AS docker-cli

FROM golang:1.25@sha256:9006890ecba0a168034d99516084099ae3114d9f2b7d6572c77f2dde57ebc980

ARG KIND_VERSION=v0.32.0
ARG KIND_SHA256=50030de23cf40a18505f20426f6a8506bedf13c6e509244bd1fa9463721b0f54
ARG KUBECTL_VERSION=v1.34.1
ARG KUBECTL_SHA256=7721f265e18709862655affba5343e85e1980639395d5754473dafaadcaa69e3

COPY --from=docker-cli /usr/local/bin/docker /usr/local/bin/docker
COPY --from=docker-cli /usr/local/libexec/docker/cli-plugins/ /usr/local/libexec/docker/cli-plugins/

RUN curl -fsSLo /usr/local/bin/kind \
        "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-linux-amd64" \
    && curl -fsSLo /usr/local/bin/kubectl \
        "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    && echo "${KIND_SHA256}  /usr/local/bin/kind" | sha256sum --check --status \
    && echo "${KUBECTL_SHA256}  /usr/local/bin/kubectl" | sha256sum --check --status \
    && chmod 0555 /usr/local/bin/kind /usr/local/bin/kubectl \
    && kind version \
    && kubectl version --client

WORKDIR /src
