package v1beta1

/*
Copyright 2022 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// An InputSpec specifies the environment for rendering composed
// resources.
type InputSpec struct {
	// DefaultData statically defines the initial state of the environment.
	// It has the same schema-less structure as the data field in
	// environment configs.
	// It is overwritten by the selected environment configs.
	DefaultData map[string]extv1.JSON `json:"defaultData,omitempty"`

	// EnvironmentConfigs selects a list of `EnvironmentConfig`s. The resolved
	// resources are stored in the composite resource at
	// `spec.environmentConfigRefs` and is only updated if it is null.
	//
	// The list of references is used to compute an in-memory environment at
	// compose time. The data of all object is merged in the order they are
	// listed, meaning the values of EnvironmentConfigs with a larger index take
	// priority over ones with smaller indices.
	//
	// The computed environment can be accessed in a composition using
	// `FromEnvironmentFieldPath` and `CombineFromEnvironment` patches.
	// +optional
	EnvironmentConfigs []EnvironmentSource `json:"environmentConfigs,omitempty"`

	// Policy represents the Resolution policy which apply to all
	// EnvironmentSourceReferences in EnvironmentConfigs list.
	// +optional
	Policy *Policy `json:"policy,omitempty"`

	// DataOverrides allows overriding the resulting environment data with
	// static values. The keys are field paths in the environment data, and the
	// values are references to where the data should be taken from (e.g. "spec.parameters.foo")
	DataOverrides map[string]string `json:"dataOverrides,omitempty"`
}

// Policy represents the Resolution policy of Reference instance.
type Policy struct {
	// Resolve specifies when this reference should be resolved. The default
	// is 'IfNotPresent', which will attempt to resolve the reference only when
	// the corresponding field is not present. Use 'Always' to resolve the
	// reference on every reconcile.
	// +optional
	// +kubebuilder:validation:Enum=Always;IfNotPresent
	// TODO(phisco): we could consider implementing IfNotPresent, but it will
	// 	require environmentConfigRefs to be moved to the XR's status, as
	// 	Functions can not write to the XR spec. Right now we behave as if
	// 	Always was set.
	//Resolve *xpv1.ResolvePolicy `json:"resolve,omitempty"`

	// Resolution specifies whether resolution of this reference is required.
	// The default is 'Required', which means the reconcile will fail if the
	// reference cannot be resolved. 'Optional' means this reference will be
	// a no-op if it cannot be resolved.
	// +optional
	// +kubebuilder:default=Required
	// +kubebuilder:validation:Enum=Required;Optional
	Resolution *xpv1.ResolutionPolicy `json:"resolution,omitempty"`
}

// IsResolutionPolicyOptional checks whether the resolution policy of relevant
// reference is Optional.
func (p *Policy) IsResolutionPolicyOptional() bool {
	if p == nil || p.Resolution == nil {
		return false
	}

	return *p.Resolution == xpv1.ResolutionPolicyOptional
}

// EnvironmentSourceType specifies the way the EnvironmentConfig is selected.
type EnvironmentSourceType string

const (
	// EnvironmentSourceTypeReference by name.
	EnvironmentSourceTypeReference EnvironmentSourceType = "Reference"
	// EnvironmentSourceTypeSelector by labels.
	EnvironmentSourceTypeSelector EnvironmentSourceType = "Selector"
)

// EnvironmentSource selects a EnvironmentConfig resource.
type EnvironmentSource struct {
	// Type specifies the way the EnvironmentConfig is selected.
	// Default is `Reference`
	// +optional
	// +kubebuilder:validation:Enum=Reference;Selector
	// +kubebuilder:default=Reference
	Type EnvironmentSourceType `json:"type,omitempty"`

	// Ref is a named reference to a single EnvironmentConfig.
	// Either Ref or Selector is required.
	// +optional
	Ref *EnvironmentSourceReference `json:"ref,omitempty"`

	// Selector selects EnvironmentConfig(s) via labels.
	// +optional
	Selector *EnvironmentSourceSelector `json:"selector,omitempty"`
}

// GetType returns the type of the environment source, returning the default if not set.
func (e *EnvironmentSource) GetType() EnvironmentSourceType {
	if e == nil || e.Type == "" {
		return EnvironmentSourceTypeReference
	}
	return e.Type
}

// An EnvironmentSourceReference references an EnvironmentConfig by it's name.
type EnvironmentSourceReference struct {
	// The name of the object.
	Name string `json:"name"`
}

// EnvironmentSourceSelectorModeType specifies amount of retrieved EnvironmentConfigs
// with matching label.
type EnvironmentSourceSelectorModeType string

const (
	// EnvironmentSourceSelectorSingleMode extracts only first EnvironmentConfig from the sorted list.
	EnvironmentSourceSelectorSingleMode EnvironmentSourceSelectorModeType = "Single"

	// EnvironmentSourceSelectorMultiMode extracts multiple EnvironmentConfigs from the sorted list.
	EnvironmentSourceSelectorMultiMode EnvironmentSourceSelectorModeType = "Multiple"
)

// An EnvironmentSourceSelector selects an EnvironmentConfig via labels.
type EnvironmentSourceSelector struct {

	// Mode specifies retrieval strategy: "Single" or "Multiple".
	// +kubebuilder:validation:Enum=Single;Multiple
	// +kubebuilder:default=Single
	Mode EnvironmentSourceSelectorModeType `json:"mode,omitempty"`

	// MaxMatch specifies the number of extracted EnvironmentConfigs in Multiple mode, extracts all if nil.
	MaxMatch *uint64 `json:"maxMatch,omitempty"`

	// MinMatch specifies the required minimum of extracted EnvironmentConfigs in Multiple mode.
	MinMatch *uint64 `json:"minMatch,omitempty"`

	// SortByFieldPath is the path to the field based on which list of EnvironmentConfigs is alphabetically sorted.
	// +kubebuilder:default="metadata.name"
	SortByFieldPath string `json:"sortByFieldPath,omitempty"`

	// MatchLabels ensures an object with matching labels is selected.
	MatchLabels []EnvironmentSourceSelectorLabelMatcher `json:"matchLabels,omitempty"`
}

func (e *EnvironmentSourceSelector) GetMode() EnvironmentSourceSelectorModeType {
	if e == nil || e.Mode == "" {
		return EnvironmentSourceSelectorSingleMode
	}
	return e.Mode
}

func (e *EnvironmentSourceSelector) GetSortByFieldPath() string {
	if e == nil || e.SortByFieldPath == "" {
		return "metadata.name"
	}
	return e.SortByFieldPath
}

// EnvironmentSourceSelectorLabelMatcherType specifies where the value for a
// label comes from.
type EnvironmentSourceSelectorLabelMatcherType string

const (
	// EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath extracts
	// the label value from a composite fieldpath.
	EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath EnvironmentSourceSelectorLabelMatcherType = "FromCompositeFieldPath"
	// EnvironmentSourceSelectorLabelMatcherTypeValue uses a literal as label
	// value.
	EnvironmentSourceSelectorLabelMatcherTypeValue EnvironmentSourceSelectorLabelMatcherType = "Value"
)

// An EnvironmentSourceSelectorLabelMatcher acts like a k8s label selector but
// can draw the label value from a different path.
type EnvironmentSourceSelectorLabelMatcher struct {
	// Type specifies where the value for a label comes from.
	// +optional
	// +kubebuilder:validation:Enum=FromCompositeFieldPath;Value
	// +kubebuilder:default=FromCompositeFieldPath
	Type EnvironmentSourceSelectorLabelMatcherType `json:"type,omitempty"`

	// Key of the label to match.
	Key string `json:"key"`

	// ValueFromFieldPath specifies the field path to look for the label value.
	ValueFromFieldPath *string `json:"valueFromFieldPath,omitempty"`

	// FromFieldPathPolicy specifies the policy for the valueFromFieldPath.
	// The default is Required, meaning that an error will be returned if the
	// field is not found in the composite resource.
	// Optional means that if the field is not found in the composite resource,
	// that label pair will just be skipped. N.B. other specified label
	// matchers will still be used to retrieve the desired
	// environment config, if any.
	// +kubebuilder:validation:Enum=Optional;Required
	// +kubebuilder:default=Required
	FromFieldPathPolicy *FromFieldPathPolicy `json:"fromFieldPathPolicy,omitempty"`

	// Value specifies a literal label value.
	Value *string `json:"value,omitempty"`
}

// FromFieldPathIsOptional returns true if the FromFieldPathPolicy is set to
// Optional.
func (e *EnvironmentSourceSelectorLabelMatcher) FromFieldPathIsOptional() bool {
	return e.FromFieldPathPolicy != nil && *e.FromFieldPathPolicy == FromFieldPathPolicyOptional
}

// GetType returns the type of the label matcher, returning the default if not set.
func (e *EnvironmentSourceSelectorLabelMatcher) GetType() EnvironmentSourceSelectorLabelMatcherType {
	if e == nil || e.Type == "" {
		return EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath
	}
	return e.Type
}

// A FromFieldPathPolicy determines how to patch from a field path.
type FromFieldPathPolicy string

// FromFieldPath patch policies.
const (
	FromFieldPathPolicyOptional FromFieldPathPolicy = "Optional"
	FromFieldPathPolicyRequired FromFieldPathPolicy = "Required"
)

// A PatchPolicy configures the specifics of patching behaviour.
type PatchPolicy struct {
	// FromFieldPath specifies how to patch from a field path. The default is
	// 'Optional', which means the patch will be a no-op if the specified
	// fromFieldPath does not exist. Use 'Required' if the patch should fail if
	// the specified path does not exist.
	// +kubebuilder:validation:Enum=Optional;Required
	// +optional
	FromFieldPath *FromFieldPathPolicy `json:"fromFieldPath,omitempty"`
	MergeOptions  *xpv1.MergeOptions   `json:"mergeOptions,omitempty"`
}

// GetFromFieldPathPolicy returns the FromFieldPathPolicy for this PatchPolicy, defaulting to FromFieldPathPolicyOptional if not specified.
func (pp *PatchPolicy) GetFromFieldPathPolicy() FromFieldPathPolicy {
	if pp == nil || pp.FromFieldPath == nil {
		return FromFieldPathPolicyOptional
	}
	return *pp.FromFieldPath
}
