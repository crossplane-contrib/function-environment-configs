# Example manifests

Before, one had to define references at `spec.environment.environmentConfigs`,
see [./old/composition.yaml]() and needed to have Crossplane actually deployed
to be able to validate that part of the logic.

The same can now be achieved using `function-environment-config`, which allows
to leverage the power of `crossplane beta render`:

```shell
$ crossplane beta render  \
  --extra-resources environmentConfigs.yaml \
  --include-context \
  xr.yaml composition.yaml functions.yaml
```

Which will output both the `Context` containing the `environment` and the `XR`
itself:

```yaml
---
apiVersion: example.crossplane.io/v1
kind: XR
metadata:
  name: example-xr
status:
  conditions:
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    reason: Available
    status: "True"
    type: Ready
  fromEnv: by-label
---
apiVersion: render.crossplane.io/v1beta1
kind: Context
fields:
  apiextensions.crossplane.io/environment:
    apiVersion: internal.crossplane.io/v1alpha1
    kind: Environment
    complex:
      a: b
      c:
        d: by-label
        f: "1"
        g: by-label
```
