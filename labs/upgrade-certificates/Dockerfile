FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce AS certificates

RUN apk add --no-cache openssl \
    && mkdir -p /out \
    && openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout /out/ca.key \
        -out /out/ca.crt \
        -days 2 \
        -subj "/CN=RavenDB operator upgrade lab CA" \
    && openssl req -newkey rsa:2048 -nodes \
        -keyout /out/server.key \
        -out /out/server.csr \
        -subj "/CN=a.lab.svc.cluster.local" \
        -addext "subjectAltName=DNS:a.lab.svc.cluster.local,DNS:b.lab.svc.cluster.local,DNS:c.lab.svc.cluster.local" \
        -addext "keyUsage=critical,digitalSignature,keyEncipherment" \
        -addext "extendedKeyUsage=serverAuth" \
    && openssl x509 -req \
        -in /out/server.csr \
        -CA /out/ca.crt \
        -CAkey /out/ca.key \
        -CAcreateserial \
        -out /out/server.crt \
        -days 2 \
        -copy_extensions copy \
    && openssl req -newkey rsa:2048 -nodes \
        -keyout /out/client.key \
        -out /out/client.csr \
        -subj "/CN=RavenDB operator upgrade lab client" \
        -addext "keyUsage=critical,digitalSignature" \
        -addext "extendedKeyUsage=clientAuth" \
    && openssl x509 -req \
        -in /out/client.csr \
        -CA /out/ca.crt \
        -CAkey /out/ca.key \
        -CAcreateserial \
        -out /out/client.crt \
        -days 2 \
        -copy_extensions copy \
    && openssl pkcs12 -export -legacy \
        -out /out/server.pfx \
        -inkey /out/server.key \
        -in /out/server.crt \
        -certfile /out/ca.crt \
        -passout pass: \
    && openssl pkcs12 -export -legacy \
        -out /out/client.pfx \
        -inkey /out/client.key \
        -in /out/client.crt \
        -certfile /out/ca.crt \
        -passout pass: \
    && rm /out/*.csr /out/*.srl /out/*.key /out/server.crt /out/client.crt

FROM scratch

COPY --from=certificates /out/ /lab-certs/
