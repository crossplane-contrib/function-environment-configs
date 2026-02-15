# Example manifests

You can run your function locally and test it using `crossplane beta render`
with these example manifests.

This example demonstrates four different transform types on label selectors,
each pulling from different fields of the composite resource:

1. **Regexp group extract** — `spec.region` (`us-east-1-abc`) → capture group 1
   → `us-east-1`
2. **Regexp replace with backreferences** —
   `metadata.labels[crossplane.io/claim-namespace]` (`team-alpha`) →
   `${1}-environment-config` → `alpha-environment-config`
3. **Map transform** — `spec.tier` (`production`) → mapped to `prod`
4. **Convert to lowercase** — `spec.priority` (`CRITICAL`) → `critical`

Together they select the EnvironmentConfig labeled `region: us-east-1`,
`config: alpha-environment-config`, `tier: prod`, `priority: critical`.

```shell
# Run the function locally
$ go run . --insecure --debug
```

```shell
# Then, in another terminal, call it with these example manifests
$ crossplane render \
  --extra-resources selector_with_transforms/environmentConfigs.yaml \
  --include-context \
  selector_with_transforms/xr.yaml selector_with_transforms/composition.yaml selector_with_transforms/functions.yaml
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
  fromEnv: https://us-east-1.example.com
  team: alpha
---
apiVersion: render.crossplane.io/v1beta1
fields:
  apiextensions.crossplane.io/environment:
    apiVersion: internal.crossplane.io/v1alpha1
    kind: Environment
    region:
      endpoint: https://us-east-1.example.com
      name: us-east-1
    team: alpha
kind: Context
```
