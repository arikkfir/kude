# label

This function adds/updates label(s) of incoming resources. Label values can be provided verbatim in function
configuration or read from a file.

## Usage

```yaml
apiVersion: kude.kfirs.com/v1alpha2
kind: Pipeline
resources:
  - deployment.yaml
steps:
  - image: ghcr.io/arikkfir/kude/functions/label
    config:
      name: app.kubernetes.io/name
      value: my-app
  - image: ghcr.io/arikkfir/kude/functions/label
    config:
      name: app.kubernetes.io/version
      path: VERSION
```

The pipeline above would add the `app.kubernetes.io/name` and `app.kubernetes.io/version` annotations to all resources 
in the `deployment.yaml` manifest. The value for the `app.kubernetes.io/name` label would be `kude-example` and the
value for the `app.kubernetes.io/version` label would be taken from the `VERSION` file.
