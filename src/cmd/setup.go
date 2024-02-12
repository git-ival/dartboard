/*
Copyright © 2024 SUSE LLC

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

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/moio/scalability-tests/pkg/kubectl"
	"github.com/moio/scalability-tests/pkg/terraform"
	"github.com/urfave/cli/v2"
)

const (
	argTerraformDir     = "tf-dir"
	argTerraformVarFile = "tf-var-file"
	baseDir             = ""
	adminPassword       = "adminadminadmin"
)

var (
	tfProvider = "unknown"
)

func main() {
	app := &cli.App{
		Usage:     "test Rancher at scale",
		Copyright: "(c) 2024 SUSE LLC",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    argTerraformDir,
				Value:   filepath.Join(baseDir, "terraform", "main", "k3d"),
				EnvVars: []string{"TERRAFORM_WORK_DIR"},
				Action: func(cCtx *cli.Context, tfDir string) error {
					tfProvider = filepath.Base(tfDir)
					return nil
				},
			},
		},
		Commands: []*cli.Command{
			{
				Name:        "setup",
				Usage:       "Deploys the test environment",
				Description: "prepares the test environment deploying the clusters and installing the required charts",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    argTerraformVarFile,
						Value:   "",
						EnvVars: []string{"TERRAFORM_VAR_FILE"},
					},
				},
				Action: actionCmdSetup,
			},
			{
				Name:        "state",
				Usage:       "Retrieves information of the deployed clusters",
				Description: "print out the state of the provisioned clusters",
				Action:      actionCmdState,
			},
			{
				Name:        "teardown",
				Aliases:     []string{"destroy"},
				Usage:       "Tears down the test environment (all the clusters)",
				Description: "destroy all the provisioned clusters",
				Action:      actionCmdDestroy,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func actionCmdState(cCtx *cli.Context) error {
	tf := new(terraform.Terraform)

	if err := tf.Init(cCtx.String(argTerraformDir), false); err != nil {
		return err
	}

	clusters, err := tf.OutputClustersJson()
	if err != nil {
		return err
	}

	fmt.Println(clusters)
	return nil
}

func actionCmdDestroy(cCtx *cli.Context) error {
	tf := new(terraform.Terraform)

	if err := tf.Init(cCtx.String(argTerraformDir), true); err != nil {
		return err
	}

	return tf.Destroy(cCtx.String(argTerraformVarFile))
}

func actionCmdSetup(cCtx *cli.Context) error {
	// Terraform
	tf := new(terraform.Terraform)

	if err := tf.Init(cCtx.String(argTerraformDir), true); err != nil {
		return err
	}
	terraformVersionPrint(tf)

	if err := tf.Apply(cCtx.String(argTerraformVarFile)); err != nil {
		return err
	}

	clusters, err := tf.OutputClusters()
	if err != nil {
		return err
	}

	// Helm charts
	tester := clusters["tester"]

	if err := chartInstallMimir(&tester); err != nil {
		return err
	}
	if err := chartInstallK6Files(&tester); err != nil {
		return err
	}
	if err := chartInstallGrafanaDashboard(&tester); err != nil {
		return err
	}
	if err := chartInstallGrafana(&tester); err != nil {
		return err
	}

	upstream := clusters["upstream"]
	// TODO: implement "importImage" function

	if err := chartInstallCertManager(&upstream); err != nil {
		return err
	}
	if err := chartInstallRancher(&upstream); err != nil {
		return err
	}
	if err := chartInstallRancherIngress(&upstream); err != nil {
		return err
	}
	if err := chartInstallRancherMonitoring(&upstream); err != nil {
		return err
	}
	if err := chartInstallCgroupsExporter(&upstream); err != nil {
		return err
	}

	// Import downstream clusters
	// TODO: Wait on the Rancher Deployment to be complete, or the import of downstream clusters may fail
	cliTester := kubectl.Client{}
	if err := cliTester.Init(tester.Kubeconfig); err != nil {
		return err
	}

	add, err := getAppAddressFor(upstream)
	if err != nil {
		return err
	}

	downstreamClusters := []string{}
	for clusterName := range clusters {
		if strings.HasPrefix(clusterName, "downstream") {
			downstreamClusters = append(downstreamClusters, clusterName)
		}
	}
	importedClusterNames := strings.Join(downstreamClusters, ",")
	fmt.Println(importedClusterNames)

	envVars := map[string]string{
		"BASE_URL":               add.Public.HTTPSURL,
		"BOOTSTRAP_PASSWORD":     "admin",
		"PASSWORD":               adminPassword,
		"IMPORTED_CLUSTER_NAMES": importedClusterNames,
	}

	if err := cliTester.K6run(envVars, nil, "k6/rancher_setup.js", true, false); err != nil {
		return err
	}

	return nil
}

func isProviderK3d() bool {
	return tfProvider == "k3d"
}

func terraformVersionPrint(tf *terraform.Terraform) error {
	ver, providers, err := tf.Version()
	if err != nil {
		return err
	}

	log.Printf("Terraform version: %s", ver)
	log.Printf("provider list:")
	for prov, ver := range providers {
		log.Printf("- %s (%s)", prov, ver)
	}
	return nil
}