# configmap

This function generates a `Namespace` resource.

## Usage

```yaml
apiVersion: kude.kfirs.com/v1alpha2
kind: Pipeline
resources:
  - deployment.yaml
steps:
  - image: ghcr.io/arikkfir/kude/functions/create-namespace
    config:
      name: my-namespace
```

The above Kude package would yield the following YAML:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: my-namespace
```
