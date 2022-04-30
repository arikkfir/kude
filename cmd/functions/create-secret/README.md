# secret

This function generates a `Secret` resource from a set of given key/value mappings.

Values for each key can be provided verbatim in function configuration or read from a file.

The `Secret` can also be marked as `immutable` if desired (see Kubernetes documentation on what immutable `Secret` 
resources are).

## Usage

```yaml
apiVersion: kude.kfirs.com/v1alpha1
kind: Pipeline
resources:
  - deployment.yaml
pipeline:
  - image: ghcr.io/arikkfir/kude/functions/create-secret:latest
    config:
      name: my-secret
      namespace: appX # optional
      immutable: true # optional (default in Kubernetes is false)
      contents:
        - key: foo
          value: bar
        - key: file-foo
          path: FOO.txt
```

Assuming the file `FOO.txt` contains `file-bar`, the pipeline above would add a `Secret` resource that would look like
this:

```yaml
apiVersion: v1
data:
  file-foo: ZmlsZS1iYXI=
  foo: YmFy
immutable: true
kind: Secret
metadata:
  annotations:
    kude.kfirs.com/previous-name: my-secret
  name: my-secret-248ydjh28y42h3 # made-up hash here, mileage will vary
  namespace: appX
```

The values in the `data` map are base64 encoded as required by Kubernetes (this is not encryption!).
