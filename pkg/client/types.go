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
	"io/fs"
	"io/ioutil"
	"os"
	"sync"

	"github.com/ghodss/yaml"
	"github.com/keikoproj/cluster-validator/pkg/api/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type Validator struct {
	sync.RWMutex
	Waiter
	Validation       *v1alpha1.ClusterValidation
	Kubernetes       dynamic.Interface
	ClusterResources map[string][]unstructured.Unstructured
}

type Waiter struct {
	sync.WaitGroup
	finished chan bool
	errors   chan error
	summary  chan ValidationSummary
}

type ConditionValidationResult struct {
	Condition      string
	ResourceErrors map[string][]string
}

func NewConditionValidationResult(cond string) ConditionValidationResult {
	return ConditionValidationResult{
		Condition:      cond,
		ResourceErrors: make(map[string][]string),
	}
}

type FieldValidationResult struct {
	FieldPath      string
	ResourceErrors map[string][]string
}

func NewFieldValidationResult(path string) FieldValidationResult {
	return FieldValidationResult{
		FieldPath:      path,
		ResourceErrors: make(map[string][]string),
	}
}

type ValidationSummary struct {
	GVR                 schema.GroupVersionResource
	FieldValidation     []FieldValidationResult
	ConditionValidation []ConditionValidationResult
}

func (v *Validator) GetResources() []v1alpha1.ClusterResource {
	return v.Validation.Spec.Resources
}

func ParseValidationSpec(path string) (*v1alpha1.ClusterValidation, error) {
	validationSpec := &v1alpha1.ClusterValidation{}
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return validationSpec, errors.Errorf("path '%v' does not exist", path)
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return validationSpec, errors.Errorf("could not read file '%v': %v", path, err)
	}

	if err := yaml.Unmarshal(data, validationSpec); err != nil {
		return validationSpec, errors.Errorf("failed to unmarshal manifest file: %v", err)
	}

	return validationSpec, nil
}

func NewValidator(c dynamic.Interface, m *v1alpha1.ClusterValidation) *Validator {
	v := &Validator{
		Waiter: Waiter{
			finished: make(chan bool),
			errors:   make(chan error),
			summary:  make(chan ValidationSummary),
		},
		Validation:       m,
		Kubernetes:       c,
		ClusterResources: make(map[string][]unstructured.Unstructured),
	}

	for _, r := range m.Spec.Resources {
		v.ClusterResources[r.Name] = make([]unstructured.Unstructured, 0)
	}

	return v
}

type ValidationStatus string

type ValidationError struct {
	Status    ValidationStatus
	Message   error
	Summaries []ValidationSummary
}
