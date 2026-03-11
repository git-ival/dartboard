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

package subcommands

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rancher/dartboard/internal/dart"
	"github.com/rancher/dartboard/internal/kubectl"
	"github.com/rancher/dartboard/internal/tofu"
	cli "github.com/urfave/cli/v2"
)

const K6_THRESHOLDS_EXCEEDED_EXIT_CODE int = 99

// TODO: Make this command idempotent. Get count (# of resources) matching some unique identifier.
// Then rerun the appropriate script, passing in the index to leave off on.
// * Scripts need to support this type of idempotency, they currently do not
func Load(cli *cli.Context) error {
	tf, r, err := prepare(cli)
	if err != nil {
		return err
	}

	clusters, _, err := tf.ParseOutputs()
	if err != nil {
		return err
	}

	// Refresh k6 files
	tester := clusters["tester"]
	if err := chartInstall(tester.Kubeconfig, chart{"k6-files", "tester", "k6-files"}, nil); err != nil {
		return err
	}

	// Create ConfigMaps and Secrets on Rancher and all the downstream clusters
	for clusterName, clusterData := range clusters {
		// NOTE: we may change the condition with 'cluster == "tester"', but better to stay on the safe side
		if clusterName != "upstream" && !strings.HasPrefix(clusterName, "downstream") {
			continue
		}

		err = loadConfigMapAndSecrets(r, tester.Kubeconfig, clusterName, clusterData)
		if k6err := handleK6RunError(err, fmt.Sprintf("loading ConfigMaps and Secrets on cluster %q", clusterName)); k6err != nil {
			return k6err
		}
	}

	// Create Users and Roles
	err = loadRolesAndUsers(r, tester.Kubeconfig, "upstream", clusters["upstream"])
	if k6err := handleK6RunError(err, "loading Roles and Users on cluster \"upstream\""); k6err != nil {
		return k6err
	}
	// Create Projects
	err = loadProjects(r, tester.Kubeconfig, "upstream", clusters["upstream"])
	if k6err := handleK6RunError(err, "loading Projects on cluster \"upstream\""); k6err != nil {
		return k6err
	}

	return nil
}

func loadConfigMapAndSecrets(r *dart.Dart, kubeconfig string, clusterName string, clusterData tofu.Cluster) error {
	configMapCount := strconv.Itoa(r.TestVariables.TestConfigMaps)
	secretCount := strconv.Itoa(r.TestVariables.TestSecrets)

	envVars := map[string]string{
		"BASE_URL":         clusterData.KubernetesAddresses.Private,
		"KUBECONFIG":       clusterData.Kubeconfig,
		"CONTEXT":          clusterData.Context,
		"CONFIG_MAP_COUNT": configMapCount,
		"SECRET_COUNT":     secretCount,
	}
	tags := map[string]string{
		"cluster":    clusterName,
		"test":       "create_k8s_resources.js",
		"ConfigMaps": configMapCount,
		"Secrets":    secretCount,
	}

	log.Printf("Load resources on cluster %q (#ConfigMaps: %s, #Secrets: %s)\n", clusterName, configMapCount, secretCount)

	return kubectl.K6run(kubeconfig, "generic/create_k8s_resources.js", envVars, tags, true, clusterData.KubernetesAddresses.Tunnel, false)
}

func loadRolesAndUsers(r *dart.Dart, kubeconfig string, clusterName string, clusterData tofu.Cluster) error {
	roleCount := strconv.Itoa(r.TestVariables.TestRoles)
	userCount := strconv.Itoa(r.TestVariables.TestUsers)

	clusterAdd, err := getAppAddressFor(clusterData)
	if err != nil {
		return fmt.Errorf("failed loading Roles and Users on cluster %q: %w", clusterName, err)
	}

	envVars := map[string]string{
		"BASE_URL":      clusterAdd.Public.HTTPSURL,
		"USERNAME":      "admin",
		"PASSWORD":      r.ChartVariables.AdminPassword,
		"USER_PASSWORD": r.ChartVariables.UserPassword,
		"ROLE_COUNT":    roleCount,
		"USER_COUNT":    userCount,
	}
	tags := map[string]string{
		"cluster": clusterName,
		"test":    "create_roles_users.mjs",
		"Roles":   roleCount,
		"Users":   userCount,
	}

	log.Printf("Load resources on cluster %q (#Roles: %s, #Users: %s)\n", clusterName, roleCount, userCount)

	return kubectl.K6run(kubeconfig, "generic/create_roles_users.js", envVars, tags, true, clusterAdd.Local.HTTPSURL, false)
}

func loadProjects(r *dart.Dart, kubeconfig string, clusterName string, clusterData tofu.Cluster) error {
	projectCount := strconv.Itoa(r.TestVariables.TestProjects)

	clusterAdd, err := getAppAddressFor(clusterData)
	if err != nil {
		return fmt.Errorf("failed loading Projects on cluster %q: %w", clusterName, err)
	}

	envVars := map[string]string{
		"BASE_URL":      clusterAdd.Public.HTTPSURL,
		"USERNAME":      "admin",
		"PASSWORD":      r.ChartVariables.AdminPassword,
		"PROJECT_COUNT": projectCount,
	}
	tags := map[string]string{
		"cluster":  clusterName,
		"test":     "create_projects.mjs",
		"Projects": projectCount,
	}

	log.Printf("Load resources on cluster %q (#Projects: %s)\n", clusterName, projectCount)

	return kubectl.K6run(kubeconfig, "generic/create_projects.js", envVars, tags, true, clusterAdd.Local.HTTPSURL, false)
}

func handleK6RunError(err error, message string) error {
	if err == nil {
		return nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		log.Printf("k6 run error for %s: %v (%v)\n", message, err, exitErr)
		// k6 exits with 99 on threshold failures. This is not a fatal error for the load command.
		if exitErr.ExitCode() == K6_THRESHOLDS_EXCEEDED_EXIT_CODE {
			log.Printf("k6 thresholds exceeded (exit code %d) for %s, continuing...", K6_THRESHOLDS_EXCEEDED_EXIT_CODE, message)
			return nil
		}
	} else {
		log.Printf("k6 run error for %s: %v\n", message, err)
	}

	// Fallback: Check error string in case error wrapping prevented errors.As from working
	if strings.Contains(err.Error(), fmt.Sprintf("exit status %d", K6_THRESHOLDS_EXCEEDED_EXIT_CODE)) {
		log.Printf("k6 thresholds exceeded (detected via error string) for %s, continuing...", message)
		return nil
	}

	return fmt.Errorf("failed %s: %w", message, err)
}
