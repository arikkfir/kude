# annotate

This function adds/updates annotation(s) of incoming resources. Annotation values can be provided verbatim in function
configuration or read from a file.

## Usage

```yaml
apiVersion: kude.kfirs.com/v1alpha1
kind: Pipeline
resources:
  - deployment.yaml
pipeline:
  - image: ghcr.io/arikkfir/kude/functions/annotate
    config:
      name: purpose
      value: kude-example
  - image: ghcr.io/arikkfir/kude/functions/annotate
    config:
      name: copyright
      path: COPYRIGHT
```

The pipeline above would add the `purpose` and `copyright` annotations to all resources in the `deployment.yaml`
manifest. The value for the `purpose` annotation would be `kude-example` and the value for the `copyright` annotation
would be taken from the `COPYRIGHT` file.
