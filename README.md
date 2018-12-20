# PodSet Operator

A simple Kubernetes [Operator][sdk_url] to manage a set of Pods.

## Overview

The PodSet is a simple Kubernetes Operator built using the [Operator SDK][sdk_url]. It's purpose is to demostrate a simple Operator that manages a set of Pods and will scale up and down based on the desired number. The operator will also update it's status with the names of the managed Pods.

## Usage

The first step is to setup the Service Account and RBAC for the operator.

```
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
```

Next, install the PodSet Resource that the operator will manage.

```
kubectl create -f deploy/crds/operator_v1alpha1_podset_crd.yaml
```

Now deploy the operator.

```
kubectl create -f deploy/operator.yaml
```

Finally, a new PodSet can be created.

```
kubectl create -f deploy/crds/operator_v1alpha1_podset_crd.yaml
```

## Development

First, follow the [Operator SDK guide][sdk_quick_start] to set up the framework locally.

Clone the repository locally and ensure dependencies are present.

```
git clone <REPO_URL>
cd podset-operator
dep ensure
```

Make any desired changes and build a new container image.

```
operator-sdk build <REPO_URL>/podset-operator
```

## License

The PodSet Operator is released under the [Apache 2.0 license][license_file].

[license_file]:./LICENSE
[sdk_url]:https://github.com/operator-framework/operator-sdk
[sdk_quick_start]:https://github.com/operator-framework/operator-sdk/tree/v0.2.1#quick-start