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
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	"github.com/keikoproj/cluster-validator/pkg/api/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Validator struct {
	sync.RWMutex
	Waiter
	Validation       *v1alpha1.ClusterValidation
	Kubernetes       dynamic.Interface
	RESTClient       *rest.RESTClient
	HTTPClient       *http.Client
	ClusterResources map[string][]unstructured.Unstructured
}

type Waiter struct {
	sync.WaitGroup
	finished chan bool
	errors   chan error
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

type HTTPEndpointValidationResult struct {
	Errors map[string]string
	Name   string
}

func NewHTTPEndpointValidationResult(name string) HTTPEndpointValidationResult {
	return HTTPEndpointValidationResult{
		Errors: make(map[string]string),
		Name:   name,
	}
}

type ClusterEndpointValidationResult struct {
	Errors map[string]string
	Name   string
}

func NewClusterEndpointValidationResult(name string) ClusterEndpointValidationResult {
	return ClusterEndpointValidationResult{
		Errors: make(map[string]string),
		Name:   name,
	}
}

type ValidationSummary struct {
	FieldValidation           []FieldValidationResult
	ConditionValidation       []ConditionValidationResult
	ClusterEndpointValidation []ClusterEndpointValidationResult
	HTTPEndpointValidation    []HTTPEndpointValidationResult
}

func (v *Validator) GetValidationObjects() []interface{} {
	objs := make([]interface{}, 0)
	for _, res := range v.GetResources() {
		objs = append(objs, res)
	}
	ep := v.GetEndpointSpec()
	for _, clusterEndpoint := range ep.Cluster {
		objs = append(objs, clusterEndpoint)
	}
	for _, httpEndpoint := range ep.HTTP {
		objs = append(objs, httpEndpoint)
	}
	return objs
}

func (v *Validator) GetResources() []v1alpha1.ClusterResource {
	return v.Validation.Spec.Resources
}

func (v *Validator) GetEndpointSpec() v1alpha1.EndpointsSpec {
	return v.Validation.Spec.Endpoints
}

func (v *Validator) GetGlobalConfiguration() v1alpha1.ValidationConfiguration {
	return v.Validation.Spec.Configuration
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

func NewValidator(c dynamic.Interface, m *v1alpha1.ClusterValidation, r *rest.RESTClient) *Validator {
	v := &Validator{
		Waiter: Waiter{
			finished: make(chan bool),
			errors:   make(chan error),
		},
		Validation: m,
		Kubernetes: c,
		RESTClient: r,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		ClusterResources: make(map[string][]unstructured.Unstructured),
	}

	for _, r := range m.Spec.Resources {
		v.ClusterResources[r.Name] = make([]unstructured.Unstructured, 0)
	}

	return v
}

type ValidationError struct {
	Message                    error
	GVR                        schema.GroupVersionResource
	FieldValidations           []FieldValidationResult
	ConditionValidations       []ConditionValidationResult
	ClusterEndpointValidations []ClusterEndpointValidationResult
	HTTPEndpointValidations    []HTTPEndpointValidationResult
}

func ToValidationError(err error) ValidationError {
	return err.(ValidationError)
}

func (e ValidationError) Error() string {
	fieldValidationResult, _ := json.MarshalIndent(e.FieldValidations, "", "\t")
	conditionValidationResult, _ := json.MarshalIndent(e.ConditionValidations, "", "\t")
	return fmt.Sprintf("%v.\nGVR: %s/%s/%s.\nField Validation Results: %s\nCondition Validation Results: %s", e.Message,
		e.GVR.Group, e.GVR.Version, e.GVR.Resource, string(fieldValidationResult), string(conditionValidationResult))
}
