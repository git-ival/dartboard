package actions

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/shepherd/clients/harvester"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/shepherd/pkg/session"
	harvesteraction "github.com/rancher/tests/interoperability/harvester"
)

type HarvesterImportClient struct {
	client          *rancher.Client
	session         *session.Session
	harvesterClient *harvester.Client
	clusterID       string
	host            string
}

func NewHarvesterConfig(host, adminToken, adminPassword string, insecure bool) harvester.Config {
	defaultBool := false

	return harvester.Config{
		Host:          host,
		AdminToken:    adminToken,
		AdminPassword: adminPassword,
		Insecure:      &insecure,
		Cleanup:       &defaultBool,
	}
}

// Function to import the Harvester client into the Rancher cluster
func NewHarvesterImportClient(rancherClient *rancher.Client, harvesterConfig *harvester.Config) (*HarvesterImportClient, error) {
	h := HarvesterImportClient{
		client:  rancherClient,
		session: session.NewSession(),
		host:    harvesterConfig.Host,
	}

	harvesterClient, err := harvester.NewClientForConfig(harvesterConfig.AdminToken, harvesterConfig, h.session)
	if err != nil {
		return nil, fmt.Errorf("error while setting up Harvester client: %v", err)
	}

	h.harvesterClient = harvesterClient

	h.session.RegisterCleanupFunc(func() error {
		return harvesteraction.ResetHarvesterRegistration(h.harvesterClient)
	})

	return &h, nil
}

// ImportCluster imports the Harvester cluster into Rancher and sets up the necessary credentials for it.
// Note: This function currently only registers the Harvester cluster with Rancher. Additional steps may
// be needed to fully set up the cluster in Rancher, such as generating a kubeconfig and creating cloud credentials.
func (h *HarvesterImportClient) ImportCluster() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	return h.ImportClusterWithContext(ctx)
}

func (h *HarvesterImportClient) ImportClusterWithContext(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	resource := h.harvesterClient.Steve.SteveType("harvesterhci.io.setting")

	setting, err := resource.ByID("cluster-registration-url")
	if err != nil {
		return err
	}

	var current struct {
		Value string `json:"value"`
	}
	if err := v1.ConvertToK8sType(setting.JSONResp, &current); err != nil {
		return err
	}

	provisioningClient, err := h.client.GetKubeAPIProvisioningClient()
	if err != nil {
		return err
	}

	existing, err := provisioningClient.Clusters(fleetNamespace).List(ctx, metav1.ListOptions{LabelSelector: "provider.cattle.io=harvester"})
	if err != nil {
		return err
	}

	for _, cluster := range existing.Items {
		if current.Value == "" || cluster.Status.ClusterName == "" {
			continue
		}

		token, err := registrationToken(ctx, h.client, cluster.Status.ClusterName)
		if err != nil {
			return err
		}

		if token.ManifestURL == current.Value {
			h.clusterID = cluster.Status.ClusterName
			_, err := waitForCluster(ctx, h.client, cluster.Name, func(cluster *provv1.Cluster) bool { return cluster.Status.Ready })

			return err
		}
	}

	digest := sha256.Sum256([]byte(h.host))

	name := fmt.Sprintf("dartboard-harvester-%x", digest[:6])
	if _, err := ensureCluster(ctx, h.client, &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: fleetNamespace, Labels: map[string]string{"provider.cattle.io": "harvester"}},
	}); err != nil {
		return err
	}

	cluster, err := waitForCluster(ctx, h.client, name, func(cluster *provv1.Cluster) bool { return cluster.Status.ClusterName != "" })
	if err != nil {
		return err
	}

	token, err := registrationToken(ctx, h.client, cluster.Status.ClusterName)
	if err != nil {
		return err
	}

	if _, err := resource.Update(setting, map[string]interface{}{"value": token.ManifestURL}); err != nil {
		return err
	}

	h.clusterID = cluster.Status.ClusterName
	logrus.Infof("Registered Harvester cluster %s", h.clusterID)
	_, err = waitForCluster(ctx, h.client, name, func(cluster *provv1.Cluster) bool { return cluster.Status.Ready })

	return err
}
