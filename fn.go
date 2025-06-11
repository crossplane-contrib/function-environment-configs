package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"google.golang.org/protobuf/types/known/structpb"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"

	"github.com/crossplane-contrib/function-environment-configs/input/v1beta1"
)

const (
	// FunctionContextKeyEnvironment is a well-known Context key where the computed Environment
	// will be stored, so that Crossplane v1 and other functions can access it, e.g. function-patch-and-transform.
	FunctionContextKeyEnvironment = "apiextensions.crossplane.io/environment"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) { //nolint:gocyclo // TODO(phisco): refactor
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	if in.Spec.EnvironmentConfigs == nil {
		f.log.Debug("No EnvironmentConfigs specified, exiting")
		return rsp, nil
	}

	oxr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get observed composite resource"))
		return rsp, nil
	}

	// Note(phisco): We need to compute the selectors even if we already
	// requested them already at the previous iteration.
	requirements, err := buildRequirements(in, oxr)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot build requirements"))
		return rsp, nil
	}

	rsp.Requirements = requirements

	if req.ExtraResources == nil {
		f.log.Debug("No extra resources specified, exiting", "requirements", rsp.GetRequirements())
		return rsp, nil
	}

	var inputEnv *unstructured.Unstructured
	if v, ok := request.GetContextKey(req, FunctionContextKeyEnvironment); ok {
		inputEnv = &unstructured.Unstructured{}
		if err := resource.AsObject(v.GetStructValue(), inputEnv); err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot get Composition environment from %T context key %q", req, FunctionContextKeyEnvironment))
			return rsp, nil
		}
		f.log.Debug("Loaded Composition environment from Function context", "context-key", FunctionContextKeyEnvironment)
	}

	extraResources, err := request.GetExtraResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	envConfigs, err := getSelectedEnvConfigs(in, extraResources)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get selected environment configs"))
		return rsp, nil
	}

	mergedData, err := mergeEnvConfigsData(envConfigs)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot merge environment data"))
		return rsp, nil
	}

	// merge input env if any (merged EnvironmentConfigs data  > default data > input env)
	if inputEnv != nil {
		mergedData = mergeMaps(inputEnv.Object, mergedData)
	}

	// merge default data if any (merged EnvironmentConfigs data  > default data > input env)
	if in.Spec.DefaultData != nil {
		defaultData, err := unmarshalData(in.Spec.DefaultData)
		if err != nil {
			response.Fatal(rsp, errors.Wrapf(err, "cannot unmarshal default data"))
			return rsp, nil
		}
		mergedData = mergeMaps(defaultData, mergedData)
	}

	// build environment and return it in the response as context
	out := &unstructured.Unstructured{Object: mergedData}
	if out.GroupVersionKind().Empty() {
		out.SetGroupVersionKind(schema.GroupVersionKind{Group: "internal.crossplane.io", Kind: "Environment", Version: "v1alpha1"})
	}
	v, err := resource.AsStruct(out)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot convert Composition environment to protobuf Struct well-known type"))
		return rsp, nil
	}
	f.log.Debug("Computed Composition environment", "environment", v)
	response.SetContextKey(rsp, FunctionContextKeyEnvironment, structpb.NewStructValue(v))

	return rsp, nil
}

func getSelectedEnvConfigs(in *v1beta1.Input, extraResources map[string][]resource.Extra) (envConfigs []unstructured.Unstructured, err error) {
	for i, config := range in.Spec.EnvironmentConfigs {
		extraResName := fmt.Sprintf("environment-config-%d", i)
		resources, ok := extraResources[extraResName]
		if !ok {
			return nil, errors.Errorf("cannot find expected extra resource %q", extraResName)
		}
		switch config.GetType() {
		case v1beta1.EnvironmentSourceTypeReference:
			out, err := processSourceByReference(in, config, resources)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot process environment config %q by reference, %q", config.Ref.Name, extraResName)
			}
			if out == nil {
				continue
			}
			envConfigs = append(envConfigs, *out)

		case v1beta1.EnvironmentSourceTypeSelector:
			out, err := processEnvironmentSource(config, resources)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot process environment config %q by selector", extraResName)
			}
			if len(out) > 0 {
				envConfigs = append(envConfigs, out...)
			}
		}
	}
	return envConfigs, nil
}

func processEnvironmentSource(config v1beta1.EnvironmentSource, resources []resource.Extra) ([]unstructured.Unstructured, error) {
	out := make([]unstructured.Unstructured, 0)
	selector := config.Selector
	switch selector.GetMode() {
	case v1beta1.EnvironmentSourceSelectorSingleMode:
		if len(resources) != 1 {
			return nil, errors.Errorf("expected exactly one extra resource, got %d", len(resources))
		}
		out = append(out, *resources[0].Resource)
	case v1beta1.EnvironmentSourceSelectorMultiMode:
		if selector.MinMatch != nil && uint64(len(resources)) < *selector.MinMatch {
			return nil, errors.Errorf("expected at least %d extra resources, got %d", *selector.MinMatch, len(resources))
		}
		if err := sortExtrasByFieldPath(resources, selector.GetSortByFieldPath()); err != nil {
			return nil, err
		}
		if selector.MaxMatch != nil && uint64(len(resources)) > *selector.MaxMatch {
			resources = resources[:*selector.MaxMatch]
		}
		for _, r := range resources {
			out = append(out, *r.Resource)
		}
	default:
		// should never happen
		return nil, errors.Errorf("unknown selector mode %q", selector.Mode)
	}
	return out, nil
}

func processSourceByReference(in *v1beta1.Input, config v1beta1.EnvironmentSource, resources []resource.Extra) (*unstructured.Unstructured, error) {
	envConfigName := config.Ref.Name
	if len(resources) == 0 {
		if in.Spec.Policy.IsResolutionPolicyOptional() {
			return nil, nil
		}
		return nil, errors.Errorf("Required environment config %q not found", envConfigName)
	}
	if len(resources) > 1 {
		return nil, errors.Errorf("expected exactly one extra resource %q, got %d", envConfigName, len(resources))
	}
	return resources[0].Resource, nil
}

func sortExtrasByFieldPath(extras []resource.Extra, path string) error { //nolint:gocyclo // TODO(phisco): refactor
	if path == "" {
		return errors.New("cannot sort by empty field path")
	}
	p := make([]struct {
		ec  resource.Extra
		val any
	}, len(extras))

	var t reflect.Type
	for i := range extras {
		p[i].ec = extras[i]
		val, err := fieldpath.Pave(extras[i].Resource.Object).GetValue(path)
		if err != nil && !fieldpath.IsNotFound(err) {
			return err
		}
		p[i].val = val
		if val == nil {
			continue
		}
		vt := reflect.TypeOf(val)
		switch {
		case t == nil:
			t = vt
		case t != vt:
			return errors.Errorf("cannot sort values of different types %q and %q", t, vt)
		}
	}
	if t == nil {
		// we either have no values or all values are nil, we can just return
		return nil
	}

	var err error
	sort.Slice(p, func(i, j int) bool {
		vali, valj := p[i].val, p[j].val
		if vali == nil {
			vali = reflect.Zero(t).Interface()
		}
		if valj == nil {
			valj = reflect.Zero(t).Interface()
		}
		switch t.Kind() { //nolint:exhaustive // we only support these types
		case reflect.Float64:
			return vali.(float64) < valj.(float64)
		case reflect.Float32:
			return vali.(float32) < valj.(float32)
		case reflect.Int64:
			return vali.(int64) < valj.(int64)
		case reflect.Int32:
			return vali.(int32) < valj.(int32)
		case reflect.Int16:
			return vali.(int16) < valj.(int16)
		case reflect.Int8:
			return vali.(int8) < valj.(int8)
		case reflect.Int:
			return vali.(int) < valj.(int)
		case reflect.String:
			return vali.(string) < valj.(string)
		default:
			// should never happen
			err = errors.Errorf("unsupported type %q for sorting", t)
			return false
		}
	})
	if err != nil {
		return err
	}

	for i := 0; i < len(extras); i++ {
		extras[i] = p[i].ec
	}
	return nil
}

func buildRequirements(in *v1beta1.Input, xr *resource.Composite) (*fnv1.Requirements, error) {
	extraResources := make(map[string]*fnv1.ResourceSelector, len(in.Spec.EnvironmentConfigs))
	for i, config := range in.Spec.EnvironmentConfigs {
		extraResName := fmt.Sprintf("environment-config-%d", i)
		switch config.Type {
		case v1beta1.EnvironmentSourceTypeReference, "":
			extraResources[extraResName] = &fnv1.ResourceSelector{
				ApiVersion: "apiextensions.crossplane.io/v1beta1",
				Kind:       "EnvironmentConfig",
				Match: &fnv1.ResourceSelector_MatchName{
					MatchName: config.Ref.Name,
				},
			}
		case v1beta1.EnvironmentSourceTypeSelector:
			matchLabels := map[string]string{}
			for _, selector := range config.Selector.MatchLabels {
				switch selector.GetType() {
				case v1beta1.EnvironmentSourceSelectorLabelMatcherTypeValue:
					// TODO validate value not to be nil
					matchLabels[selector.Key] = *selector.Value
				case v1beta1.EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath:
					value, err := fieldpath.Pave(xr.Resource.Object).GetString(*selector.ValueFromFieldPath)
					if err != nil {
						if !selector.FromFieldPathIsOptional() {
							return nil, errors.Wrapf(err, "cannot get value from field path %q", *selector.ValueFromFieldPath)
						}
						continue
					}
					matchLabels[selector.Key] = value
				}
			}
			if len(matchLabels) == 0 {
				continue
			}
			extraResources[extraResName] = &fnv1.ResourceSelector{
				ApiVersion: "apiextensions.crossplane.io/v1beta1",
				Kind:       "EnvironmentConfig",
				Match: &fnv1.ResourceSelector_MatchLabels{
					MatchLabels: &fnv1.MatchLabels{Labels: matchLabels},
				},
			}
		}
	}
	return &fnv1.Requirements{ExtraResources: extraResources}, nil
}

func mergeEnvConfigsData(configs []unstructured.Unstructured) (map[string]interface{}, error) {
	merged := map[string]interface{}{}
	for _, c := range configs {
		data := map[string]interface{}{}
		if err := fieldpath.Pave(c.Object).GetValueInto("data", &data); err != nil {
			return nil, errors.Wrapf(err, "cannot get data from environment config %q", c.GetName())
		}

		merged = mergeMaps(merged, data)
	}
	return merged, nil
}

func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}

func unmarshalData(data map[string]extv1.JSON) (map[string]interface{}, error) {
	res := map[string]interface{}{}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	return res, nil
}
