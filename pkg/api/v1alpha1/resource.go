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

package v1alpha1

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type ClusterResource struct {
	Name       string `json:"name"`
	APIVersion string `json:"apiVersion"`
	Required   bool   `json:"required"`

	Configuration ValidationConfiguration `json:"configuration,omitempty"`
	Namespaces    *SelectionScope         `json:"namespaces,omitempty"`
	Names         *SelectionScope         `json:"names,omitempty"`
	Fields        []FieldSelector         `json:"fields,omitempty"`
	Annotations   []AnnotationSelector    `json:"annotations,omitempty"`
	Conditions    []ResourceCondition     `json:"conditions,omitempty"`
}

func (c *ClusterResource) GetConfiguration() ValidationConfiguration {
	return c.Configuration
}

type FieldSelector struct {
	Path   string   `json:"path"`
	Values []string `json:"values"`
}

func (f *FieldSelector) GetPath() string {
	s := strings.Split(f.Path, "=")
	return s[0]
}

func (f *FieldSelector) GetValues() []string {
	if len(f.Values) > 0 {
		return f.Values
	}
	return []string{"*"}
}

type AnnotationOperator string

const (
	AnnotationOperatorExists AnnotationOperator = "Exists"
	AnnotationOperatorEqual  AnnotationOperator = "Equal"
)

type AnnotationSelector struct {
	Key      string             `json:"key"`
	Value    string             `json:"value,omitempty"`
	Operator AnnotationOperator `json:"operator,omitempty"`
}

type SelectionScope struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

type ResourceCondition struct {
	Type   string                 `json:"type,omitempty"`
	Status corev1.ConditionStatus `json:"status,omitempty"`
	Path   string                 `json:"path,omitempty"`
}
