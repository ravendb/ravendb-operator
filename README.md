

# ravendb operator -  deployment guide

### 1. Build & Push the Operator Image

If you modified the operator source code, do the following (using your own registry account):

```
make generate
make manifests
make docker-build IMG=thegoldenplatypus/ravendb-operator-multi-node:latest
```

Since we are currently working on a demo and using a lot of automatic generation, mistakes can happen.
I recommend scanning the extracted fs of the image before pushing:

```
trufflehog filesystem extracted-fs
trivy fs ./extracted-fs
```

> it's also reccomended to scan before pushing to gh:
> ` ggshield secret scan pre-commit`
> or (before committing)
> `ggshield secret scan pre-commit`

Push the image:
```
docker push thegoldenplatypus/ravendb-operator-multi-node:latest

```

### 2. Create Kubernetes Cluster

Until we figure out the networking part of deployment on multi-node, I’m mainly testing on a
single-node environment like Minikube:

```
minikube start
```

### 3. Prepare for Operator Deployment

* NOTE: skip step 3 if you want to setup insecure cluster, and notice the note comment in step 7.

Assuming you already obtained a setup package, unzip it and have it on your controller machine.
Create namespace and secrets (adjust the paths as needed):

```
kubectl create namespace ravendb
kubectl create secret generic ravendb-certs-a --from-file=server.pfx=./misc/multinode_https/setup_package/A/cluster.server.certificate.thegoldenplatypus.pfx -n ravendb
kubectl create secret generic ravendb-certs-b --from-file=server.pfx=./misc/multinode_https/setup_package/B/cluster.server.certificate.thegoldenplatypus.pfx -n ravendb
kubectl create secret generic ravendb-certs-c --from-file=server.pfx=./misc/multinode_https/setup_package/C/cluster.server.certificate.thegoldenplatypus.pfx -n ravendb

```



### 4. Deploy Ingress Controller

For your convenience, our custom NGINX controller manifest can be found under the misc/ dir:

```
kubectl apply -f ./misc/nginx-ingress-ravendb.yaml
```

If you are using Minikube like me, you can open a new shell and run: minikube tunnel
If you prefer Kind/Docker Desktop port forwarding is also fine.
For VMs/bare metal use MetalLB.

Whatever you choose, make sure that at the end you have assigned an external IP to your ingress controller:
```
$ kubectl get svc -A
NAMESPACE       NAME                        TYPE           CLUSTER-IP      EXTERNAL-IP     PORT(S)                      AGE
ingress-nginx   ingress-nginx-controller    LoadBalancer   10.96.240.149   10.96.240.149   80:31836/TCP,443:32683/TCP   69s

```


### 5. Update /etc/hosts

```
$ sudo nano /etc/hosts

# replace <IP> with your ingress external IP and adjust the SNIs based on your setup Package
<IP> a.thegoldenplatypus.development.run a-tcp.thegoldenplatypus.development.run
<IP> b.thegoldenplatypus.development.run b-tcp.thegoldenplatypus.development.run
<IP> c.thegoldenplatypus.development.run c-tcp.thegoldenplatypus.development.run
```


### 6. Deploy Operator 
`make deploy IMG=thegoldenplatypus/ravendb-operator-multi-node:latest`

```
$ kubectl get pods -A
NAMESPACE                 NAME                                                   READY   STATUS      RESTARTS   AGE
ravendb-operator-system   ravendb-operator-controller-manager-7f66f94b8f-hjwmc   1/1     Running     0          2m13s
```

### 7. Modify and Deploy CR

* NOTE: want to setup insecure cluster use the commented definition in: https://github.com/TheGoldenPlatypus/ravendb-operator/blob/main/config/samples/ravendb_v1alpha1_ravendbcluster.yaml.

Edit by: `nano config/samples/ravendb_v1alpha1_ravendbcluster.yaml`

```
apiVersion: ravendb.ravendb.io/v1alpha1
kind: RavenDBCluster
metadata:
  labels:
    app.kubernetes.io/name: ravendb-operator
    app.kubernetes.io/managed-by: kustomize
  name: ravendbcluster-sample
  namespace: ravendb
spec:
  nodes:
    - name: a
      publicServerUrl: https://a.thegoldenplatypus.development.run:443
      publicServerUrlTcp: tcp://a-tcp.thegoldenplatypus.development.run:443
      certsSecretRef: ravendb-certs-a
    - name: b
      publicServerUrl: https://b.thegoldenplatypus.development.run:443
      publicServerUrlTcp: tcp://b-tcp.thegoldenplatypus.development.run:443
      certsSecretRef: ravendb-certs-b
    - name: c
      publicServerUrl: https://c.thegoldenplatypus.development.run:443
      publicServerUrlTcp: tcp://c-tcp.thegoldenplatypus.development.run:443
      certsSecretRef: ravendb-certs-c

  image: ravendb/ravendb:latest
  imagePullPolicy: IfNotPresent
  mode: LetsEncrypt
  email: omer.ratsaby@ravendb.net
  license: ''
  domain: thegoldenplatypus.development.run
  serverUrl: https://0.0.0.0:443
  serverUrlTcp: tcp://0.0.0.0:38888
  storageSize: 5Gi
  ingressClassName: nginx

```
* Don’t forget to provide your license.

Deploy by: `kubectl apply -f config/samples/ravendb_v1alpha1_ravendbcluster.yaml -n ravendb
`

### 8. Verify Deployment

```
$ kubectl get pods -A
NAMESPACE                 NAME                                                   READY   STATUS      RESTARTS   AGE
ravendb-operator-system   ravendb-operator-controller-manager-7f66f94b8f-hjwmc   1/1     Running     0          2m13s
ravendb                   ravendb-a-0                                            1/1     Running     0          40s
ravendb                   ravendb-b-0                                            1/1     Running     0          39s
ravendb                   ravendb-c-0                                            1/1     Running     0          39s

$ kubectl get svc -A
NAMESPACE                 NAME                                                  TYPE           CLUSTER-IP      EXTERNAL-IP     PORT(S)                      AGE
ravendb                   ravendb-a                                             ClusterIP      10.104.39.107   <none>          443/TCP,38888/TCP            43s
ravendb                   ravendb-b                                             ClusterIP      10.97.173.253   <none>          443/TCP,38888/TCP            42s
ravendb                   ravendb-c                                             ClusterIP      10.96.203.191   <none>          443/TCP,38888/TCP            42s

$ kubectl get ingress -A
NAMESPACE   NAME        CLASS   HOSTS                                                                         ADDRESS         PORTS   AGE
ravendb     ravendb-a   nginx   a.thegoldenplatypus.development.run,a-tcp.thegoldenplatypus.development.run   10.96.240.149   80      43s
ravendb     ravendb-b   nginx   b.thegoldenplatypus.development.run,b-tcp.thegoldenplatypus.development.run   10.96.240.149   80      43s
ravendb     ravendb-c   nginx   c.thegoldenplatypus.development.run,c-tcp.thegoldenplatypus.development.run   10.96.240.149   80      43s
```

### 9.  Access RavenDB Server and Form the Cluster

* Adjust to match your paths:
```
# extract necessery files from the bundel

sudo openssl pkcs12 -in ./misc/multinode_httpss/setup_package/A/cluster.server.certificate.thegoldenplatypus.pfx -clcerts -nokeys -out ./misc/multinode_httpss/setup_package/A/cluster.server.certificate.pem -legacy -passin pass:
sudo openssl pkcs12 -in ./misc/multinode_https/setup_package/A/cluster.server.certificate.thegoldenplatypus.pfx -nocerts -nodes -out ./misc/multinode_https/setup_package/A/cluster.server.certificate.key -legacy -passin pass:
sudo chmod 640 ./misc/multinode_https/setup_package/admin.client.certificate.thegoldenplatypus.pfx

# put  admin client cert to the A node

sudo curl -X PUT "https://a.thegoldenplatypus.development.run/admin/certificates" \
-H "Content-Type: application/json" \
--cert ./misc/multinode_https/setup_package/A/cluster.server.certificate.pem \
--key ./misc/multinode_https/setup_package/A/cluster.server.certificate.key \
-d '{ "Name": "AdminClientCert", "Certificate": "'"$(cat ./misc/multinode_https/setup_package/admin.client.certificate.thegoldenplatypus.pfx | base64 -w 0)"'", "SecurityClearance": "ClusterAdmin" }'

# form the cluster
sudo curl --cert ./misc/multinode_https/setup_package/A/cluster.server.certificate.pem --key ./misc/multinode_https/setup_package/A/cluster.server.certificate.key -L -X PUT https://a.thegoldenplatypus.development.run/admin/cluster/node?url=https%3A%2F%2Fb.thegoldenplatypus.development.run&tag=B
sudo curl --cert ./misc/multinode_https/setup_package/A/cluster.server.certificate.pem --key ./misc/multinode_https/setup_package/A/cluster.server.certificate.key -L -X PUT https://a.thegoldenplatypus.development.run/admin/cluster/node?url=https%3A%2F%2Fc.thegoldenplatypus.development.run&tag=C

```

### 10.  Verify Cluster Topology

```
sudo curl --cert ./misc/multinode_https/setup_package/A/cluster.server.certificate.pem --key ./misc/multinode_httpss/setup_package/A/cluster.server.certificate.key "https://a.thegoldenplatypus.development.run/cluster/topology" | jq -c '.Topology.AllNodes'

{"A":"https://a.thegoldenplatypus.development.run:443","B":"https://b.thegoldenplatypus.development.run","C":"https://c.thegoldenplatypus.development.run"}

```


## Tests
 
Integration tests + args validation tests were added to the repo. as we grow over time we will need to implement e2e tests which are the ultimate tests.

integration tests:

```
$ go test -v ./internal/controller/...
=== RUN   TestControllers
Running Suite: controller Suite - /mnt/c/Users/omer.ratsaby/Desktop/RavenDB-24283/ravendb-operator/internal/controller
======================================================================================================================
Random Seed: 1749122903

Will run 2 of 2 specs

Ran 2 of 2 Specs in 7.681 seconds
SUCCESS! -- 2 Passed | 0 Failed | 0 Pending | 0 Skipped
--- PASS: TestControllers (7.68s)
PASS
ok      ravendb-operator/internal/controller    7.709s

```

args validation tests:
```
$ go test -v ./api/v1alpha1/...
=== RUN   TestEmailValidation
=== RUN   TestEmailValidation/valid_email
=== RUN   TestEmailValidation/empty_email
=== RUN   TestEmailValidation/invalid_email_format
    validation_test.go:112: RavenDBCluster.ravendb.ravendb.io "test-invalid-email-format" is invalid: spec.email: Invalid value: "bademail": spec.email in body should match '^[^@\s]+@[^@\s]+\.[^@\s]+$'
--- PASS: TestEmailValidation (2.02s)
    --- PASS: TestEmailValidation/valid_email (2.01s)
    --- PASS: TestEmailValidation/empty_email (0.00s)
    --- PASS: TestEmailValidation/invalid_email_format (0.00s)
=== RUN   TestImageValidation
=== RUN   TestImageValidation/valid_image
=== RUN   TestImageValidation/missing_image
    validation_test.go:112: RavenDBCluster.ravendb.ravendb.io "test-missing-image" is invalid: spec.image: Invalid value: "": spec.image in body should be at least 1 chars long
--- PASS: TestImageValidation (0.00s)
    --- PASS: TestImageValidation/valid_image (0.00s)
    --- PASS: TestImageValidation/missing_image (0.00s)
=== RUN   TestImagePullPolicyValidation
=== RUN   TestImagePullPolicyValidation/valid_pull_policy_Always
=== RUN   TestImagePullPolicyValidation/valid_pull_policy_IfNotPresent
=== RUN   TestImagePullPolicyValidation/invalid_pull_policy
    validation_test.go:112: RavenDBCluster.ravendb.ravendb.io "test-invalid-pull-policy" is invalid: spec.imagePullPolicy: Unsupported value: "InvalidPolicy": supported values: "Always", "IfNotPresent", "Never"
--- PASS: TestImagePullPolicyValidation (0.01s)
    --- PASS: TestImagePullPolicyValidation/valid_pull_policy_Always (0.00s)
    --- PASS: TestImagePullPolicyValidation/valid_pull_policy_IfNotPresent (0.00s)
    --- PASS: TestImagePullPolicyValidation/invalid_pull_policy (0.00s)
=== RUN   TestModeValidation
=== RUN   TestModeValidation/valid_mode_None
=== RUN   TestModeValidation/valid_mode_LetsEncrypt
=== RUN   TestModeValidation/invalid_mode
    validation_test.go:112: RavenDBCluster.ravendb.ravendb.io "test-invalid-mode" is invalid: spec.mode: Unsupported value: "InvalidMode": supported values: "None", "LetsEncrypt"
--- PASS: TestModeValidation (0.01s)
    --- PASS: TestModeValidation/valid_mode_None (0.00s)
    --- PASS: TestModeValidation/valid_mode_LetsEncrypt (0.01s)
    --- PASS: TestModeValidation/invalid_mode (0.00s)
=== RUN   TestLicenseValidation
=== RUN   TestLicenseValidation/valid_license
=== RUN   TestLicenseValidation/missing_license
    validation_test.go:112: RavenDBCluster.ravendb.ravendb.io "test-missing-license" is invalid: spec.license: Invalid value: "": spec.license in body should be at least 1 chars long
--- PASS: TestLicenseValidation (0.00s)
    --- PASS: TestLicenseValidation/valid_license (0.00s)
    --- PASS: TestLicenseValidation/missing_license (0.00s)
=== RUN   TestDomainValidation
=== RUN   TestDomainValidation/valid_domain
=== RUN   TestDomainValidation/missing_domain
    validation_test.go:112: RavenDBCluster.ravendb.ravendb.io "test-missing-domain" is invalid: spec.domain: Invalid value: "": spec.domain in body should be at least 1 chars long
--- PASS: TestDomainValidation (0.01s)
    --- PASS: TestDomainValidation/valid_domain (0.00s)
    --- PASS: TestDomainValidation/missing_domain (0.00s)
=== RUN   TestServerUrlValidation
=== RUN   TestServerUrlValidation/valid_server_url
=== RUN   TestServerUrlValidation/missing_server_url
    validation_test.go:112: RavenDBCluster.ravendb.ravendb.io "test-missing-server-url" is invalid: spec.serverUrl: Invalid value: "": spec.serverUrl in body should be at least 1 chars long
--- PASS: TestServerUrlValidation (0.01s)
    --- PASS: TestServerUrlValidation/valid_server_url (0.00s)
    --- PASS: TestServerUrlValidation/missing_server_url (0.00s)
=== RUN   TestServerUrlTcpValidation
=== RUN   TestServerUrlTcpValidation/valid_server_url_tcp
=== RUN   TestServerUrlTcpValidation/missing_server_url_tcp
    validation_test.go:112: RavenDBCluster.ravendb.ravendb.io "test-missing-server-url-tcp" is invalid: spec.serverUrlTcp: Invalid value: "": spec.serverUrlTcp in body should be at least 1 chars long
--- PASS: TestServerUrlTcpValidation (0.00s)
    --- PASS: TestServerUrlTcpValidation/valid_server_url_tcp (0.00s)
    --- PASS: TestServerUrlTcpValidation/missing_server_url_tcp (0.00s)
=== RUN   TestStorageSizeValidation
=== RUN   TestStorageSizeValidation/valid_storage_size
=== RUN   TestStorageSizeValidation/missing_storage_size
    validation_test.go:112: RavenDBCluster.ravendb.ravendb.io "test-missing-storage-size" is invalid: spec.storageSize: Invalid value: "": spec.storageSize in body should be at least 1 chars long
--- PASS: TestStorageSizeValidation (0.01s)
    --- PASS: TestStorageSizeValidation/valid_storage_size (0.00s)
    --- PASS: TestStorageSizeValidation/missing_storage_size (0.00s)
=== RUN   TestNodesValidation
=== RUN   TestNodesValidation/valid_nodes
=== RUN   TestNodesValidation/missing_nodes
    validation_test.go:108: skipping...
--- PASS: TestNodesValidation (0.01s)
    --- PASS: TestNodesValidation/valid_nodes (0.00s)
    --- SKIP: TestNodesValidation/missing_nodes (0.00s)
=== RUN   TestIngressClassNameValidation
=== RUN   TestIngressClassNameValidation/valid_ingress_class_name
=== RUN   TestIngressClassNameValidation/missing_ingress_class_name
    validation_test.go:112: RavenDBCluster.ravendb.ravendb.io "test-missing-ingress-class-name" is invalid: spec.ingressClassName: Invalid value: "": spec.ingressClassName in body should be at least 1 chars long
--- PASS: TestIngressClassNameValidation (0.01s)
    --- PASS: TestIngressClassNameValidation/valid_ingress_class_name (0.00s)
    --- PASS: TestIngressClassNameValidation/missing_ingress_class_name (0.00s)
PASS
ok      ravendb-operator/api/v1alpha1   (cached)
```


<!--

# ravendb-operator
// TODO(user): Add simple overview of use/purpose

## Description
// TODO(user): An in-depth paragraph about your project and overview of use

## Getting Started

### Prerequisites
- go version v1.22.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/ravendb-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/ravendb-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/ravendb-operator:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/ravendb-operator/<tag or branch>/dist/install.yaml
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

-->
