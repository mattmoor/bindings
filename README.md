# Experimental `FooBinding` type

This repo contains a proof-of-concept controller that uses Knative duck-typing
to perform "binding" akin to Service Bindings in Cloud Foundry.

This repository defines a trivial `FooBinding` CRD, which looks like:

```yaml
apiVersion: bindings.mattmoor.dev/v1alpha1
kind: FooBinding
metadata:
  name: test-binding
spec:
  # This defines the target resource into which we will inject the binding.
  target:
    apiVersion: apps/v1
    kind: Deployment
    name: debug

  # This defines the trivial payload that we will inject.
  value: OMFG it works
```

This will direct the binding controller to inject an environment variable as-if
the user had written the following in their K8s deployment:

```yaml
env:
- name: FOO
  value: OMFG it works
```

## Extensibility

This controller works with any resource that implements the "pod speccable" duck
type, which means it follows the common shape of embedding a pod spec at
`.spec.template.spec`.  To enable it on new types you first must give the
binding controller access by expanding the aggregated cluster role:

```yaml
# This piece of the aggregated cluster role enables us to bind to
# Knative serving resources
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: binding-system-knative-serving
  labels:
    bindings.mattmoor.dev/controller: "true"
rules:
  - apiGroups: ["serving.knative.dev"]
    resources: ["services", "configurations"]
    verbs: ["get", "list", "patch", "watch"]
```

After that, you can start to use bindings with the new resource types:

```yaml
apiVersion: bindings.mattmoor.dev/v1alpha1
kind: FooBinding
metadata:
  name: test-binding
spec:
  # This defines the target resource into which we will inject the binding.
  target:
    apiVersion: serving.knative.dev/v1
    kind: Service
    name: runtime

  # This defines the trivial payload that we will inject.
  value: OMFG it works with Knative Services too!!!!1!!
```

## TODO

 - Teach the webhook to `Do()` the transformations so they are applied before
  the initial mutation lands in etcd.

 - Use this (in another repo) to implement a non-trivial binding, e.g. Sink
  injection akin to the knative/eventing ContainerSource.
