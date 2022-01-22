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

package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/keikoproj/cluster-validator/pkg/client"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "validate validates a given cluster",
	Run: func(cmd *cobra.Command, args []string) {
		if specFile == "" {
			log.Fatal("--filename is required")
		}

		spec, err := client.ParseValidationSpec(specFile)
		if err != nil {
			log.Fatalf("failed to parse validation spec from file: %v", err)
		}

		c, err := GetKubernetesDynamicClient()
		if err != nil {
			log.Fatalf("failed to create dynamic client: %v", err)
		}

		v := client.NewValidator(c, spec)
		validationErr := v.Validate()
		if validationErr.Status != client.ValidationSucceeded {
			log.Fatalf("validation failed: %v", validationErr.Message)
		}
	},
}

var (
	specFile string
)

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringVar(&specFile, "filename", "", "Path to cluster validation manifest file (yaml)")
}

func GetKubernetesConfig() (*rest.Config, error) {
	var config *rest.Config
	config, err := rest.InClusterConfig()
	if err != nil {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		clientCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
		return clientCfg.ClientConfig()
	}
	return config, nil
}

func GetKubernetesDynamicClient() (dynamic.Interface, error) {
	var config *rest.Config
	config, err := GetKubernetesConfig()
	if err != nil {
		return nil, err
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client, nil
}
