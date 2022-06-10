# annotate

This function the namespace of incoming resources.

## Usage

```yaml
apiVersion: kude.kfirs.com/v1alpha1
kind: Pipeline
resources:
  - deployment.yaml
pipeline:
  - image: ghcr.io/arikkfir/kude/functions/set-namespace
    config:
      namespace: test
```
