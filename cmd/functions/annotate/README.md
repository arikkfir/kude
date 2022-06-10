# annotate

This function adds/updates annotation(s) of incoming resources. Annotation values can be provided verbatim in function
configuration or read from a file.

## Usage

```yaml
apiVersion: kude.kfirs.com/v1alpha1
kind: Pipeline
resources:
  - deployment.yaml
  - service-accounts.yaml
pipeline:
  - image: ghcr.io/arikkfir/kude/functions/annotate
    config:
      name: purpose
      value: kude-example
  - image: ghcr.io/arikkfir/kude/functions/annotate
    config:
      name: copyright
      path: COPYRIGHT
  - image: ghcr.io/arikkfir/kude/functions/annotate
    config:
      name: special
      value: super-duper
      includes:
        - apiVersion: v1
          kind: ServiceAccount
          name: special-sa
          namespace: my-ns
          labelSelector: app=my-app
```

The pipeline above would add the `purpose` and `copyright` annotations to all resources in the `deployment.yaml` and 
`service-accounts.yaml` manifests. The value for the `purpose` annotation would be `kude-example` and the value for the 
`copyright` annotation would be taken from the `COPYRIGHT` file.

Additionally, it would add the `special` annotation to all **ServiceAccount** objects named `special-sa` in the
namespace `my-ns` that have the label `app` with the value `my-app` in the `deployment.yaml` and `service-accounts.yaml`
manifests. This is done by the 3rd pipeline step which has an `includes` filter - which is an array of filtering specs;
each filter spec contains one or more of the mentioned fields (they are all optional).
