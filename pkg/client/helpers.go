/*
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

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gobwas/glob"
	"github.com/keikoproj/cluster-validator/pkg/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/jsonpath"
)

func groupVersionResource(groupVersion, resource string) schema.GroupVersionResource {
	var (
		group, version string
	)

	gvSplit := strings.Split(groupVersion, "/")
	switch len(gvSplit) {
	case 1:
		group = ""
		version = gvSplit[0]
	case 2:
		group = gvSplit[0]
		version = gvSplit[1]
	}

	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
}

func getJsonPathValue(u unstructured.Unstructured, jsonPath string) (string, error) {
	j := jsonpath.New("")
	j.AllowMissingKeys(true)

	if !strings.HasPrefix(jsonPath, "{") && !strings.HasSuffix(jsonPath, "}") {
		jsonPath = fmt.Sprintf("{%v}", jsonPath)
	}

	if err := j.Parse(jsonPath); err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	if err := j.Execute(buf, u.Object); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func unstructuredSlicePath(u unstructured.Unstructured, jsonPath string) ([]interface{}, bool, error) {
	splitFunction := func(c rune) bool {
		return c == '.'
	}
	statusPath := strings.FieldsFunc(jsonPath, splitFunction)

	value, f, err := unstructured.NestedSlice(u.UnstructuredContent(), statusPath...)
	if err != nil {
		return []interface{}{}, false, err
	}
	return value, f, nil
}

func patternMatch(pattern, str string) bool {
	g := glob.MustCompile(strings.ToLower(pattern))
	return g.Match(strings.ToLower(str))
}

func matchInPatterns(patterns []string, str string) bool {
	var condition bool
	for _, p := range patterns {
		if patternMatch(p, str) {
			condition = true
		}
	}
	return condition
}

func prettyPrintStruct(st interface{}) {
	s, _ := json.MarshalIndent(st, "", "\t")
	fmt.Println(string(s))
}

func inSelectionScope(s *v1alpha1.SelectionScope, str string) bool {

	if s == nil || str == "" {
		return true
	}

	var matchScope bool

	for _, includes := range s.Include {
		if patternMatch(includes, str) {
			matchScope = true
			break
		}
	}

	for _, excludes := range s.Exclude {
		if patternMatch(excludes, str) {
			matchScope = false
			break
		}
	}

	return matchScope
}

func namespacedName(r unstructured.Unstructured) string {
	if r.GetNamespace() == "" {
		return r.GetName()
	}
	return fmt.Sprintf("%v/%v", r.GetNamespace(), r.GetName())
}
