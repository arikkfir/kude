# configmap

This function generates a `ConfigMap` resource from a set of given key/value mappings.

Values for each key can be provided verbatim in function configuration or read from a file.

The `ConfigMap` can also be marked as `immutable` if desired (see Kubernetes documentation on what immutable `ConfigMap` 
resources are).

## Usage

```yaml
apiVersion: kude.kfirs.com/v1alpha1
kind: Pipeline
resources:
  - deployment.yaml
pipeline:
  - image: ghcr.io/arikkfir/kude/functions/create-configmap
    config:
      name: my-config
      namespace: appX # optional
      immutable: true # optional (default in Kubernetes is false)
      contents:
        - key: foo
          value: bar
        - key: file-foo
          path: FOO.txt
```

Assuming the file `FOO.txt` contains `file-bar`, the pipeline above would add a `ConfigMap` resource that would look
like this:

```yaml
apiVersion: v1
data:
  file-foo: file-bar
  foo: bar
immutable: true
kind: ConfigMap
metadata:
  annotations:
    kude.kfirs.com/previous-name: my-config
  name: my-config-248ydjh28y42h3 # made-up hash here, mileage will vary
  namespace: appX
```
