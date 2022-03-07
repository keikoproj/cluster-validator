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
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type ClusterEndpoint struct {
	Name          string                  `json:"name"`
	Required      bool                    `json:"required"`
	Configuration ValidationConfiguration `json:"configuration,omitempty"`
	URI           string                  `json:"uri,omitempty"`
}

type HTTPEndpoint struct {
	Name          string                  `json:"name"`
	Required      bool                    `json:"required"`
	Configuration ValidationConfiguration `json:"configuration,omitempty"`
	URL           string                  `json:"url,omitempty"`
	Codes         []int                   `json:"codes,omitempty"`
}

type ClusterResource struct {
	Name          string                  `json:"name"`
	APIVersion    string                  `json:"apiVersion"`
	Required      bool                    `json:"required"`
	Configuration ValidationConfiguration `json:"configuration,omitempty"`
	Namespaces    *SelectionScope         `json:"namespaces,omitempty"`
	Names         *SelectionScope         `json:"names,omitempty"`
	Fields        []FieldSelector         `json:"fields,omitempty"`
	Annotations   []AnnotationSelector    `json:"annotations,omitempty"`
	Conditions    []ResourceCondition     `json:"conditions,omitempty"`
}

func (r *ClusterResource) SuccessThreshold(globalCfg ValidationConfiguration) int {
	var (
		resourceCfg = r.GetConfiguration()
	)
	if resourceCfg.SuccessThreshold > 0 {
		return resourceCfg.SuccessThreshold
	}
	return globalCfg.SuccessThreshold
}

func (r *ClusterResource) FailureThreshold(globalCfg ValidationConfiguration) int {
	var (
		resourceCfg = r.GetConfiguration()
	)
	if resourceCfg.FailureThreshold > 0 {
		return resourceCfg.FailureThreshold
	}
	return globalCfg.FailureThreshold
}

func (r *HTTPEndpoint) SuccessThreshold(globalCfg ValidationConfiguration) int {
	var (
		resourceCfg = r.GetConfiguration()
	)
	if resourceCfg.SuccessThreshold > 0 {
		return resourceCfg.SuccessThreshold
	}
	return globalCfg.SuccessThreshold
}

func (r *HTTPEndpoint) FailureThreshold(globalCfg ValidationConfiguration) int {
	var (
		resourceCfg = r.GetConfiguration()
	)
	if resourceCfg.FailureThreshold > 0 {
		return resourceCfg.FailureThreshold
	}
	return globalCfg.FailureThreshold
}

func (r *ClusterEndpoint) SuccessThreshold(globalCfg ValidationConfiguration) int {
	var (
		resourceCfg = r.GetConfiguration()
	)
	if resourceCfg.SuccessThreshold > 0 {
		return resourceCfg.SuccessThreshold
	}
	return globalCfg.SuccessThreshold
}

func (r *ClusterEndpoint) FailureThreshold(globalCfg ValidationConfiguration) int {
	var (
		resourceCfg = r.GetConfiguration()
	)
	if resourceCfg.FailureThreshold > 0 {
		return resourceCfg.FailureThreshold
	}
	return globalCfg.FailureThreshold
}

func (c *ClusterResource) GetConfiguration() ValidationConfiguration {
	return c.Configuration
}

func (c *HTTPEndpoint) GetConfiguration() ValidationConfiguration {
	return c.Configuration
}

func (c *ClusterEndpoint) GetConfiguration() ValidationConfiguration {
	return c.Configuration
}

func (r *ClusterResource) Interval(globalCfg ValidationConfiguration) time.Duration {
	var (
		resourceCfg = r.GetConfiguration()
	)

	if resourceCfg.Interval != "" {
		d, err := time.ParseDuration(resourceCfg.Interval)
		if err != nil {
			log.Warnf("failed to parse duration '%v', using default of 1s", resourceCfg.Interval)
			return time.Second * 1
		}
		return d
	} else {
		d, err := time.ParseDuration(globalCfg.Interval)
		if err != nil {
			log.Warnf("failed to parse duration '%v', using default of 1s", globalCfg.Interval)
			return time.Second * 1
		}
		return d
	}
}

func (r *ClusterEndpoint) Interval(globalCfg ValidationConfiguration) time.Duration {
	var (
		resourceCfg = r.GetConfiguration()
	)

	if resourceCfg.Interval != "" {
		d, err := time.ParseDuration(resourceCfg.Interval)
		if err != nil {
			log.Warnf("failed to parse duration '%v', using default of 1s", resourceCfg.Interval)
			return time.Second * 1
		}
		return d
	} else {
		d, err := time.ParseDuration(globalCfg.Interval)
		if err != nil {
			log.Warnf("failed to parse duration '%v', using default of 1s", globalCfg.Interval)
			return time.Second * 1
		}
		return d
	}
}

func (r *HTTPEndpoint) Interval(globalCfg ValidationConfiguration) time.Duration {
	var (
		resourceCfg = r.GetConfiguration()
	)

	if resourceCfg.Interval != "" {
		d, err := time.ParseDuration(resourceCfg.Interval)
		if err != nil {
			log.Warnf("failed to parse duration '%v', using default of 1s", resourceCfg.Interval)
			return time.Second * 1
		}
		return d
	} else {
		d, err := time.ParseDuration(globalCfg.Interval)
		if err != nil {
			log.Warnf("failed to parse duration '%v', using default of 1s", globalCfg.Interval)
			return time.Second * 1
		}
		return d
	}
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
