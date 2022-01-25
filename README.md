# cluster-validator

`cluster-validator` is a tool/library for performing resource validations in parallel on a Kubernetes cluster.
For example, validating all nodes in the cluster are ready, and that all pods in kube-system are in "running" phase.

`cluster-validator` can currently validate fields, and conditions, on all standard and custom resources in your cluster.


## Validation config file

You can define your validation spec by creating a YAML file such as this
```yaml
# validation.yaml

apiVersion: v1alpha1
kind: ClusterValidator
metadata:
  name: namespace-validation
spec:

  # Global configuration
  configuration:
    # How many calls must pass / fail 
    successThreshold: 10
    failureThreshold: 10 
    # How long to wait between calls
    interval: 1s

  # Resources to validate
  resources:

  # The resource/s GVR
  - name: pods
    apiVersion: v1

    # The scope to validate - include to exclude name/namespace
    names:
      include: 
      - "*"
      exclude:
      - "debug-pod*"
    namespaces:
      include:
      - "kube-system"

    # Perform a field validation
    fields: 
    - path: .status.phase
      values:
      - running

    # This resource validation is required
    required: true
    
    # Override the global configuration
    configuration:
      successThreshold: 3
      failureThreshold: 10

  # Similarly, validate nodes using a condition validation
  - name: nodes
    apiVersion: v1
    names:
      include: 
      - "*"
    conditions: 
    - path: status.conditions
      type: ready
      status: true
    required: true
    configuration:
      successThreshold: 5
      failureThreshold: 10

```

More examples [here](docs/examples).
## Invoke from CLI

```bash
$ cluster-validator validate --filename ./validation.yaml                                    
INFO[0000] validating resource 'pods'
INFO[0000] validating resource 'nodes'
INFO[0001] validation of 'pods' successful (1/3)
INFO[0001] validation of 'nodes' successful (1/5)
INFO[0002] validation of 'pods' successful (2/3)
INFO[0002] validation of 'nodes' successful (2/5)
INFO[0004] validation of 'pods' successful (3/3)
INFO[0004] ✅  resource 'pods' validated successfully
INFO[0004] validation of 'nodes' successful (3/5)
INFO[0005] validation of 'nodes' successful (4/5)
INFO[0007] validation of 'nodes' successful (5/5)
INFO[0007] ✅  resource 'nodes' validated successfully
```

## Invoke from Code

```golang
import (
    validator "github.com/keikoproj/cluster-validator/pkg/client"
    "k8s.io/client-go/dynamic"
)

func validate(client dynamic.Interface) error {
	spec, err := validator.ParseValidationSpec("validation.yaml")
	if err != nil {
		return err
	}

	v := validator.NewValidator(client, spec)
	if err := v.Validate(); err != nil {
		return err
	}
}
```

The `error` returned by `Validate()` has structured data with information on the failed validation:

``` golang
v := validator.NewValidator(client, spec)
err := v.Validate()
if vErr, ok := err.(*validator.ValidationError); ok {

  fmt.Printf("Validation failed for %s/%s/%s.\n",
    vErr.GVR.Group, vErr.GVR.Version, vErr.GVR.Resource)

  for _, cVal := range vErr.ConditionValidations {

    fmt.Printf("Failed Condition: %s\n", cVal.Condition)

    for msg, resources := range cVal.ResourceErrors {
      fmt.Printf("Reason: %s.\nResources: %v", msg, resources)
    }
  }
}
```