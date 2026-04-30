package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/crossplane-contrib/function-environment-configs/input/v1beta1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"
)

const (
	errFmtRequiredField          = "%s is required by type %s"
	errFmtTransformAtIndex       = "transform at index %d returned error"
	errFmtTransformTypeFailed    = "%s transform could not resolve"
	errFmtTransformConfigMissing = "given transform type %s requires configuration"
	errFmtTypeNotSupported       = "transform type %s is not supported"

	errFmtMapTypeNotSupported = "type %s is not supported for map transform"
	errFmtMapNotFound         = "key %s is not found in map"
	errFmtMapInvalidJSON      = "value for key %s is not valid JSON"

	errFmtMatchPattern            = "cannot match pattern at index %d"
	errFmtMatchParseResult        = "cannot parse result of pattern at index %d"
	errMatchParseFallbackValue    = "cannot parse fallback value"
	errMatchFallbackBoth          = "cannot set both a fallback value and the fallback to input flag"
	errFmtMatchPatternTypeInvalid = "unsupported pattern type '%s'"
	errFmtMatchInputTypeInvalid   = "unsupported input type '%s'"
	errMatchRegexpCompile         = "cannot compile regexp"

	errStringTransformTypeFailed        = "type %s is not supported for string transform type"
	errStringTransformTypeFormat        = "string transform of type %s fmt is not set"
	errStringTransformTypeConvert       = "string transform of type %s convert is not set"
	errStringTransformTypeTrim          = "string transform of type %s trim is not set"
	errStringTransformTypeRegexp        = "string transform of type %s regexp is not set"
	errStringTransformTypeRegexpFailed  = "could not compile regexp"
	errStringTransformTypeRegexpNoMatch = "regexp %q had no matches for group %d"
	errStringTransformTypeReplace       = "string transform of type %s replace is not set"
	errStringConvertTypeFailed          = "type %s is not supported for string convert"
)

// ResolveTransforms applies a list of transforms sequentially to an input value.
func ResolveTransforms(transforms []v1beta1.Transform, input any) (any, error) {
	val := input
	for i, t := range transforms {
		var err error
		val, err = Resolve(t, val)
		if err != nil {
			return nil, errors.Wrapf(err, errFmtTransformAtIndex, i)
		}
	}
	return val, nil
}

// Resolve the supplied Transform.
func Resolve(t v1beta1.Transform, input any) (any, error) {
	var out any
	var err error

	switch t.Type {
	case v1beta1.TransformTypeMap:
		if t.Map == nil {
			return nil, errors.Errorf(errFmtTransformConfigMissing, t.Type)
		}
		out, err = ResolveMap(t.Map, input)
	case v1beta1.TransformTypeMatch:
		if t.Match == nil {
			return nil, errors.Errorf(errFmtTransformConfigMissing, t.Type)
		}
		out, err = ResolveMatch(t.Match, input)
	case v1beta1.TransformTypeString:
		if t.String == nil {
			return nil, errors.Errorf(errFmtTransformConfigMissing, t.Type)
		}
		out, err = ResolveString(t.String, input)
	default:
		return nil, errors.Errorf(errFmtTypeNotSupported, string(t.Type))
	}

	return out, errors.Wrapf(err, errFmtTransformTypeFailed, string(t.Type))
}

// ResolveMap resolves a Map transform.
func ResolveMap(t *v1beta1.MapTransform, input any) (any, error) {
	switch i := input.(type) {
	case string:
		p, ok := t.Pairs[i]
		if !ok {
			return nil, errors.Errorf(errFmtMapNotFound, i)
		}
		var val any
		if err := json.Unmarshal(p.Raw, &val); err != nil {
			return nil, errors.Wrapf(err, errFmtMapInvalidJSON, i)
		}
		return val, nil
	default:
		return nil, errors.Errorf(errFmtMapTypeNotSupported, fmt.Sprintf("%T", input))
	}
}

// ResolveMatch resolves a Match transform.
func ResolveMatch(t *v1beta1.MatchTransform, input any) (any, error) {
	var output any
	for i, p := range t.Patterns {
		matches, err := Matches(p, input)
		if err != nil {
			return nil, errors.Wrapf(err, errFmtMatchPattern, i)
		}
		if matches {
			if err := unmarshalJSON(p.Result, &output); err != nil {
				return nil, errors.Wrapf(err, errFmtMatchParseResult, i)
			}
			return output, nil
		}
	}

	// Fallback to input if no pattern matches and fallback to input is set
	if t.FallbackTo == v1beta1.MatchFallbackToTypeInput {
		if t.FallbackValue.Size() != 0 {
			return nil, errors.New(errMatchFallbackBoth)
		}

		return input, nil
	}

	// Use fallback value if no pattern matches (or if there are no patterns)
	if err := unmarshalJSON(t.FallbackValue, &output); err != nil {
		return nil, errors.Wrap(err, errMatchParseFallbackValue)
	}
	return output, nil
}

// Matches returns true if the pattern matches the supplied input.
func Matches(p v1beta1.MatchTransformPattern, input any) (bool, error) {
	switch p.Type {
	case v1beta1.MatchTransformPatternTypeLiteral:
		return matchesLiteral(p, input)
	case v1beta1.MatchTransformPatternTypeRegexp:
		return matchesRegexp(p, input)
	}
	return false, errors.Errorf(errFmtMatchPatternTypeInvalid, string(p.Type))
}

func matchesLiteral(p v1beta1.MatchTransformPattern, input any) (bool, error) {
	if p.Literal == nil {
		return false, errors.Errorf(errFmtRequiredField, "literal", v1beta1.MatchTransformPatternTypeLiteral)
	}
	inputStr, ok := input.(string)
	if !ok {
		return false, errors.Errorf(errFmtMatchInputTypeInvalid, fmt.Sprintf("%T", input))
	}
	return inputStr == *p.Literal, nil
}

func matchesRegexp(p v1beta1.MatchTransformPattern, input any) (bool, error) {
	if p.Regexp == nil {
		return false, errors.Errorf(errFmtRequiredField, "regexp", v1beta1.MatchTransformPatternTypeRegexp)
	}
	re, err := regexp.Compile(*p.Regexp)
	if err != nil {
		return false, errors.Wrap(err, errMatchRegexpCompile)
	}
	if input == nil {
		return false, errors.Errorf(errFmtMatchInputTypeInvalid, "null")
	}
	inputStr, ok := input.(string)
	if !ok {
		return false, errors.Errorf(errFmtMatchInputTypeInvalid, fmt.Sprintf("%T", input))
	}
	return re.MatchString(inputStr), nil
}

// unmarshalJSON is a small utility function that returns nil if j contains no
// data. json.Unmarshal seems to not be able to handle this.
func unmarshalJSON(j extv1.JSON, output *any) error {
	if len(j.Raw) == 0 {
		return nil
	}
	return json.Unmarshal(j.Raw, output)
}

// ResolveString resolves a String transform.
func ResolveString(t *v1beta1.StringTransform, input any) (string, error) { //nolint:gocyclo // This is a long but simple switch.
	switch t.Type {
	case v1beta1.StringTransformTypeFormat:
		if t.Format == nil {
			return "", errors.Errorf(errStringTransformTypeFormat, string(t.Type))
		}
		return fmt.Sprintf(*t.Format, input), nil
	case v1beta1.StringTransformTypeConvert:
		if t.Convert == nil {
			return "", errors.Errorf(errStringTransformTypeConvert, string(t.Type))
		}
		return stringConvertTransform(t.Convert, input)
	case v1beta1.StringTransformTypeTrimPrefix, v1beta1.StringTransformTypeTrimSuffix:
		if t.Trim == nil {
			return "", errors.Errorf(errStringTransformTypeTrim, string(t.Type))
		}
		return stringTrimTransform(input, t.Type, *t.Trim), nil
	case v1beta1.StringTransformTypeRegexp:
		if t.Regexp == nil {
			return "", errors.Errorf(errStringTransformTypeRegexp, string(t.Type))
		}
		return stringRegexpTransform(input, *t.Regexp)
	case v1beta1.StringTransformTypeReplace:
		if t.Replace == nil {
			return "", errors.Errorf(errStringTransformTypeReplace, string(t.Type))
		}
		return stringReplaceTransform(input, *t.Replace), nil
	default:
		return "", errors.Errorf(errStringTransformTypeFailed, string(t.Type))
	}
}

func stringConvertTransform(t *v1beta1.StringConversionType, input any) (string, error) {
	str := fmt.Sprintf("%v", input)
	switch *t {
	case v1beta1.StringConversionTypeToUpper:
		return strings.ToUpper(str), nil
	case v1beta1.StringConversionTypeToLower:
		return strings.ToLower(str), nil
	default:
		return "", errors.Errorf(errStringConvertTypeFailed, *t)
	}
}

func stringTrimTransform(input any, t v1beta1.StringTransformType, trim string) string {
	str := fmt.Sprintf("%v", input)
	if t == v1beta1.StringTransformTypeTrimPrefix {
		return strings.TrimPrefix(str, trim)
	}
	if t == v1beta1.StringTransformTypeTrimSuffix {
		return strings.TrimSuffix(str, trim)
	}
	return str
}

func stringRegexpTransform(input any, r v1beta1.StringTransformRegexp) (string, error) {
	re, err := regexp.Compile(r.Match)
	if err != nil {
		return "", errors.Wrap(err, errStringTransformTypeRegexpFailed)
	}

	str := fmt.Sprintf("%v", input)

	// If Replace is set, use ReplaceAllString with backreference support.
	if r.Replace != nil {
		return re.ReplaceAllString(str, *r.Replace), nil
	}

	groups := re.FindStringSubmatch(str)

	// Return the entire match (group zero) by default.
	g := ptr.Deref[int](r.Group, 0)
	if len(groups) == 0 || g >= len(groups) {
		return "", errors.Errorf(errStringTransformTypeRegexpNoMatch, r.Match, g)
	}

	return groups[g], nil
}

func stringReplaceTransform(input any, r v1beta1.StringTransformReplace) string {
	str := fmt.Sprintf("%v", input)
	return strings.ReplaceAll(str, r.Search, r.Replace)
}
