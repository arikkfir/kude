# kude

![Maintainer](https://img.shields.io/badge/maintainer-arikkfir-blue)
![GoVersion](https://img.shields.io/github/go-mod/go-version/arikkfir/kude.svg)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/arikkfir/kude)
[![GoReportCard](https://goreportcard.com/badge/github.com/arikkfir/kude)](https://goreportcard.com/report/github.com/arikkfir/kude)

Kude (**Ku**bernetes **De**ployment) is a tool for deploying Kubernetes applications. It stands on the shoulders of
giants such as [kustomize](https://kustomize.io/), [kpt](https://kpt.dev/) and [helm](https://helm.sh/), but unifies
them to one coherent model, while drawing the best features into one cohesive tool.

👉 [Yeah yeah, just take me to an example, I can figure it out!](#Example)

Kude is built as a pipeline, which starts by reading a set of resources, processing them by invoking a chain of
functions (more on that below), and producing an output of resources; those can be the same resources, usually enriched
in some way, and potentially new resources as well.

Each such "pipeline" is called a Kude Package - basically a directory with a `kude.yaml` file that describes the process
and optionally an additional set of Kubernetes manifests used by that pipeline. Kude packages can also include external
resources - local or remote. Those resources (referred to in the `kude.yaml` file) can be simple Kubernetes manifests,
Helm charts, or even other Kude packages. All of those can be either local or remote.

The pipeline functions are where the magic happens - each function receives the set of resources read so far, and is
responsible for doing some kind of manipulation - either enriching them, or producing new ones. The function's output
will be provided as input to the next function, and so forth. What makes Kude so extensible is the fact that each
function is a Docker image! This allows anyone to write Kude functions using any tool, language or method one wants!

### High level features

- Package inheritance & composition
  - Similar to Kustomize's overlay concept
  - Supports:
    - Local files
    - Git repositories
    - Remote URLs
    - Other Kude packages (local & remote)
    - [More](https://github.com/hashicorp/go-getter)!
- Name hashes for `ConfigMap` and `Secret` resources
  - This is a useful feature introduced in `kustomize`, where the name of a `ConfigMap` or `Secret` is suffixed with a
    hash (computed from its contents). Other resources referencing that `ConfigMap` or `Secret` are updaed to correctly
    reference the hashed-name. 
  - By doing this, whenever the contents of such `ConfigMap` or `Secret` are changed, their hash suffix (and hence their
    name) would change as well, resulting in a reload of the dependent pods.
- Extensible!
  - Kude packages are a pipeline of functions
  - Each function is just a Docker image adhering to a very (very!) simple contract (see below)
  - You can use any Kude function you want, and you can even write your own!
- Team player!
  - Can work with existing technologies such as Helm, Kustomize (coming soon!) and Kpt (coming soon!)
  - Works with `kubectl` easily - just run `kude | kubectl apply -f -` to deploy!
- Growing functions catalog
  - [See the catalog](#Kude-Functions-Catalog)

## Status

This is currently alpha, with some features still in development, and not full test coverage. We're on it! 💪

## Example

Given this directory structure:
```
└── my-kude-package
    ├── kude.yaml
    ├── deployment.yaml
    └── service.yaml
```

The `kude.yaml` contains:
```yaml
apiVersion: kude.kfirs.com/v1alpha1
kind: Pipeline
resources:
  # Define two local input resources:
  - deployment.yaml
  - service.yaml
pipeline:
  # Define just one function to process resources:
  - image: ghcr.io/arikkfir/kude/functions/annotate:latest
    config:
      name: purpose
      value: kude-example
```

The `deployment.yaml` contains:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: super-microservice
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: super-microservice
  template:
    metadata:
      labels:
        app.kubernetes.io/name: super-microservice
    spec:
      containers:
        - image: "examples/super-microservice:v1"
          name: microservice
          ports:
            - containerPort: 8080
```

The `service.yaml` contains:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: super-microservice
spec:
  ports:
    - name: http
      port: 80
      targetPort: 8080
  selector:
    app.kubernetes.io/name: test
```

Run `kude`:

```shell
/home/test/my-kude-package: $ kude
```

Expect the following output (notice the annotations for each resource):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    purpose: kude-example
  name: super-microservice
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: super-microservice
  template:
    metadata:
      labels:
        app.kubernetes.io/name: super-microservice
    spec:
      containers:
        - image: "examples/super-microservice:v1"
          name: microservice
          ports:
            - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  annotations:
    purpose: kude-example
  name: super-microservice
spec:
  ports:
    - name: http
      port: 80
      targetPort: 8080
  selector:
    app.kubernetes.io/name: test
```

## Kude Functions

The following functions are available:

- [annotate](./cmd/functions/annotate/README.md) - Annotate Kubernetes resources with metadata
- [configmap](cmd/functions/create-configmap/README.md) - Generate a Kubernetes ConfigMap
- [helm](./cmd/functions/helm/README.md) - Render a Helm chart
- [label](./cmd/functions/label/README.md) - Label Kubernetes resources
- [secret](cmd/functions/create-secret/README.md) - Generate a Kubernetes Secret
- [yq](./cmd/functions/yq/README.md) - Patch resources using `yq`

## Writing Kude Functions

Until we document this, please see the functions [source code](./cmd/functions).

## Contributing

We welcome any contributions from the community - help us out! Have a look at our 
[contribution guide](.github/CONTRIBUTING.md) for more information on how to get started on sending your first PR.
