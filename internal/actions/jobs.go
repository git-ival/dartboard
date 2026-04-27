package actions

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/dartboard/internal/dart"
	"github.com/rancher/dartboard/internal/tofu"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/cloudcredentials"
	shepherdclusters "github.com/rancher/shepherd/extensions/clusters"
	shepherddefaults "github.com/rancher/shepherd/extensions/defaults"
	shepherdwait "github.com/rancher/shepherd/pkg/wait"
	"github.com/rancher/tests/actions/machinepools"
	"github.com/rancher/tests/actions/provisioning"
	"github.com/rancher/tests/actions/reports"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
)

// ImportJob wraps a tofu.Cluster for import operations
type ImportJob struct {
	Cluster tofu.Cluster
}

// Name returns the cluster name
func (j ImportJob) Name() string { return j.Cluster.Name }

// IsCompleted checks if import has already been done
func (j ImportJob) IsCompleted(status *ClusterStatus) bool { return status.IsImported() }

// MarkCompleted marks the import as done
func (j ImportJob) MarkCompleted(status *ClusterStatus) { status.Stage = StageImported }

// FinalStage returns the final stage after import
func (j ImportJob) FinalStage() Stage { return StageImported }

// Execute performs the cluster import operation
func (j ImportJob) Execute(ctx context.Context, client *rancher.Client, config *rancher.Config) error {
	cluster := j.Cluster

	logrus.Infof("Importing cluster %s...", cluster.Name)

	importCluster := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: fleetNamespace,
		},
	}

	// Create the cluster object in Rancher
	updatedCluster, err := createAndWaitForCluster(ctx, client, config, importCluster)
	if err != nil {
		return fmt.Errorf("failed to create cluster %s: %w", cluster.Name, err)
	}

	logrus.Infof("Cluster %s created, now importing...", cluster.Name)

	// Perform the actual import
	updatedCluster, err = performClusterImport(ctx, client, cluster, importCluster)
	if err != nil {
		return fmt.Errorf("failed to import cluster %s: %w", cluster.Name, err)
	}

	logrus.Infof("Cluster %s imported, checking pod status...", updatedCluster.Name)

	// Verify pods are healthy
	podErrors := StatusPodsWithTimeout(client, updatedCluster.Status.ClusterName, shepherddefaults.OneMinuteTimeout)
	if len(podErrors) > 0 {
		errorStrings := make([]string, len(podErrors))
		for i, e := range podErrors {
			errorStrings[i] = e.Error()
		}

		return fmt.Errorf("pod errors in cluster %s: %s", updatedCluster.Status.ClusterName, strings.Join(errorStrings, "\n"))
	}

	return nil
}

// ProvisionJob wraps a dart.ClusterTemplate for provisioning operations
type ProvisionJob struct {
	Template dart.ClusterTemplate
}

// Name returns the generated cluster name
func (j ProvisionJob) Name() string { return j.Template.GeneratedName() }

// IsCompleted checks if provisioning has already been done
func (j ProvisionJob) IsCompleted(status *ClusterStatus) bool { return status.IsProvisioned() }

// MarkCompleted marks the provisioning as done
func (j ProvisionJob) MarkCompleted(status *ClusterStatus) { status.Stage = StageProvisioned }

// FinalStage returns the final stage after provisioning
func (j ProvisionJob) FinalStage() Stage { return StageProvisioned }

// Execute performs the cluster provisioning operation
func (j ProvisionJob) Execute(ctx context.Context, client *rancher.Client, _ *rancher.Config) error {
	template := j.Template
	clusterName := template.GeneratedName()

	logrus.Infof("Provisioning cluster %s...", clusterName)

	nodeProvider := CreateProvider(template.ClusterConfig.Provider)
	templateClusterConfig := ConvertConfigToClusterConfig(template.ClusterConfig)

	// Create the cluster
	clusterObject, err := provisioning.CreateProvisioningCluster(
		client,
		nodeProvider,
		cloudcredentials.CloudCredential{},
		templateClusterConfig,
		machinepools.MachineConfigs{},
		nil,
	)
	reports.TimeoutClusterReport(clusterObject, err)

	if err != nil {
		return fmt.Errorf("failed to provision cluster %s: %w", clusterName, err)
	}

	logrus.Infof("Cluster %s created, waiting for ready state...", clusterName)

	// Wait for the cluster to be ready
	fiveMinuteTimeout := int64(shepherddefaults.FiveMinuteTimeout)
	listOpts := metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterObject.ID,
		TimeoutSeconds: &fiveMinuteTimeout,
	}

	watchInterface, err := client.GetManagementWatchInterface(management.ClusterType, listOpts)
	if err != nil {
		return fmt.Errorf("failed to get watch interface for cluster %s: %w", clusterName, err)
	}

	if err := shepherdwait.WatchWait(watchInterface, shepherdclusters.IsProvisioningClusterReady); err != nil {
		reports.TimeoutClusterReport(clusterObject, err)

		return fmt.Errorf("cluster %s failed to become ready: %w", clusterName, err)
	}

	logrus.Infof("Cluster %s provisioned successfully", clusterName)

	return nil
}

// RegisterJob wraps a tofu.CustomCluster for registration operations
type RegisterJob struct {
	Template tofu.CustomCluster
}

// Name returns the cluster name
func (j RegisterJob) Name() string { return j.Template.Name }

// IsCompleted checks if registration has already been done
func (j RegisterJob) IsCompleted(status *ClusterStatus) bool { return status.IsRegistered() }

// MarkCompleted marks the registration as done
func (j RegisterJob) MarkCompleted(status *ClusterStatus) { status.Stage = StageRegistered }

// FinalStage returns the final stage after registration
func (j RegisterJob) FinalStage() Stage { return StageRegistered }

// Execute performs the custom cluster registration operation
func (j RegisterJob) Execute(ctx context.Context, client *rancher.Client, config *rancher.Config) error {
	template := j.Template
	clusterName := template.Name

	logrus.Infof("Registering custom cluster %s...", clusterName)

	provCluster := &provv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "provisioning.cattle.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: fleetNamespace,
		},
		Spec: provv1.ClusterSpec{
			KubernetesVersion: template.DistroVersion,
			DefaultPodSecurityAdmissionConfigurationTemplateName: psactRancherPrivileged,
			RKEConfig: &provv1.RKEConfig{},
		},
	}

	// Create or get existing cluster object
	clusterResp, err := createOrGetClusterObjectSimple(client, config, provCluster)
	if err != nil {
		return fmt.Errorf("failed to create/get cluster object %s: %w", clusterName, err)
	}

	logrus.Infof("Cluster object %s ready, registering nodes...", clusterName)

	// Build machine pools
	machinePools := createMachinePools(template)
	provCluster.Spec.RKEConfig.MachinePools = machinePools

	// Retry registration if SSH handshake fails
	var clusterObject *v1.SteveAPIObject

	err = BackoffWaitWithContext(ctx, 20, func(ctx context.Context) (bool, error) {
		var regErr error

		clusterObject, regErr = RegisterCustomCluster(client, clusterResp, provCluster, template.Nodes)
		if regErr != nil {
			if strings.Contains(regErr.Error(), "ssh: handshake failed") ||
				strings.Contains(regErr.Error(), "ssh: unable to authenticate") {
				logrus.Warnf("SSH handshake failed for cluster %s, retrying... Error: %v", clusterName, regErr)

				return false, nil
			}

			return false, regErr
		}

		return true, nil
	})
	reports.TimeoutClusterReport(clusterObject, err)

	if err != nil {
		return fmt.Errorf("failed to register cluster %s: %w", clusterName, err)
	}

	// Verify cluster
	if err := VerifyCluster(client, config, clusterObject); err != nil {
		return fmt.Errorf("cluster verification failed for %s: %w", clusterName, err)
	}

	logrus.Infof("Cluster %s registered successfully", clusterName)

	return nil
}

// createOrGetClusterObjectSimple creates a new cluster object or retrieves an existing one
// This is a simplified version without the runner dependency
func createOrGetClusterObjectSimple(client *rancher.Client, config *rancher.Config, provCluster *provv1.Cluster) (*v1.SteveAPIObject, error) {
	// First try to get existing cluster
	clusterResp, err := GetK3SRKE2Cluster(client, config, provCluster)
	if err == nil {
		return clusterResp, nil
	}

	// Create new cluster if not found
	clusterResp, err = CreateK3SRKE2Cluster(client, config, provCluster)
	if err != nil {
		return nil, err
	}

	// Wait for cluster to be queryable
	_, err = GetK3SRKE2Cluster(client, config, provCluster)
	if err != nil {
		return nil, err
	}

	return clusterResp, nil
}
