# annotate

This function the namespace of incoming resources.

## Usage

```yaml
apiVersion: kude.kfirs.com/v1alpha2
kind: Pipeline
resources:
  - deployment.yaml
steps:
  - image: ghcr.io/arikkfir/kude/functions/set-namespace
    config:
      namespace: test
```
