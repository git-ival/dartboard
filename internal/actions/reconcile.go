package actions

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/dartboard/internal/dart"
	"github.com/rancher/dartboard/internal/vendored"
	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/types"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	shepherdnodes "github.com/rancher/shepherd/pkg/nodes"
	rancherclusters "github.com/rancher/tests/actions/clusters"
	"github.com/rancher/tests/actions/machinepools"
	"golang.org/x/crypto/ssh"
	yaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func registrationToken(ctx context.Context, client *rancher.Client, clusterID string) (*management.ClusterRegistrationToken, error) {
	var token *management.ClusterRegistrationToken

	err := kwait.PollUntilContextTimeout(ctx, time.Second, 2*time.Minute, true, func(context.Context) (bool, error) {
		tokens, err := client.Management.ClusterRegistrationToken.List(&types.ListOpts{
			Filters: map[string]interface{}{"clusterId": clusterID},
		})
		if err != nil {
			return false, err
		}

		if len(tokens.Data) == 0 {
			return false, nil
		}

		token = &tokens.Data[0]

		return token.ManifestURL != "", nil
	})

	return token, err
}

func applyClusterImport(ctx context.Context, client *rancher.Client, kubeconfig, clusterID string) error {
	token, err := registrationToken(ctx, client, clusterID)
	if err != nil {
		return err
	}

	config := &rest.Config{TLSClientConfig: rest.TLSClientConfig{Insecure: client.RancherConfig.Insecure != nil && *client.RancherConfig.Insecure}}

	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return err
	}

	httpClient.Timeout = 2 * time.Minute

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, token.ManifestURL, nil)
	if err != nil {
		return fmt.Errorf("invalid registration manifest URL")
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("fetching registration manifest failed")
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching registration manifest: HTTP %d", response.StatusCode)
	}

	command := vendored.CommandContext(ctx, "kubectl", "--kubeconfig="+kubeconfig, "apply", "-f", "-")

	command.Stdin = io.LimitReader(response.Body, 16<<20)
	if err := command.Run(); err != nil {
		return fmt.Errorf("applying registration manifest: %w", err)
	}

	return nil
}

func createNamedProvisioningCluster(ctx context.Context, recipe *dart.Dart, client *rancher.Client, template dart.ClusterTemplate) (*provv1.Cluster, error) {
	configs, err := provisioningMachineConfigs(recipe, template)
	if err != nil {
		return nil, err
	}

	provider := CreateProvider(template.ClusterConfig.Provider)

	credential, err := ensureHarvesterCredential(ctx, recipe, client, template.GeneratedName())
	if err != nil {
		return nil, err
	}

	name := template.GeneratedName()
	poolConfigs := provider.MachinePoolFunc(configs, name+"-pool-", fleetNamespace)

	var responses []v1.SteveAPIObject

	for index, pool := range poolConfigs {
		pool.SetGenerateName("")
		pool.SetName(fmt.Sprintf("%s-pool-%d", name, index))

		resource := client.Steve.SteveType(provider.MachineConfigPoolResourceSteveType)

		existing, err := resource.ByID(fleetNamespace + "/" + pool.GetName())
		if err != nil {
			if !isSteveNotFound(err) {
				return nil, err
			}

			existing, err = resource.Create(&pool)
			if err != nil {
				return nil, err
			}
		}

		responses = append(responses, *existing)
	}

	config := ConvertConfigToClusterConfig(template.ClusterConfig)
	config.KubernetesVersion = template.DistroVersion

	var (
		roles []machinepools.MachinePoolConfig
		pools []machinepools.Pools
	)

	for _, pool := range config.MachinePools {
		roles = append(roles, pool.MachinePoolConfig)
		pools = append(pools, pool.Pools)
	}

	clusterPools := machinepools.CreateAllMachinePools(roles, pools, responses, provider.GetMachineRolesFunc(configs), nil)
	desired := rancherclusters.NewK3SRKE2ClusterConfig(name, fleetNamespace, config, clusterPools, credential.Namespace+":"+credential.Name)

	return ensureCluster(ctx, client, desired)
}

func isSteveNotFound(err error) bool {
	return clientbase.IsNotFound(err)
}

func provisioningMachineConfigs(recipe *dart.Dart, template dart.ClusterTemplate) (machinepools.MachineConfigs, error) {
	configs := machinepools.MachineConfigs{}
	if template.ClusterConfig.Provider != dart.HarvesterProvider {
		return configs, fmt.Errorf("provider %q has no machine configuration in this recipe; use custom clusters or supply a Harvester node configuration", template.ClusterConfig.Provider)
	}

	configs.HarvesterMachineConfigs = &machinepools.HarvesterMachineConfigs{}

	var network struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	}

	encodedNetwork, err := yaml.Marshal(recipe.TofuVariables["network"])
	if err != nil {
		return configs, err
	}

	if err := yaml.Unmarshal(encodedNetwork, &network); err != nil {
		return configs, err
	}

	if network.Name == "" {
		return configs, fmt.Errorf("Harvester provisioning requires tofu_variables.network.name")
	}

	sshUser, _ := recipe.TofuVariables["ssh_user"].(string)

	for _, pool := range template.ClusterConfig.MachinePools {
		if pool.MachinePoolConfig.Quantity <= 0 {
			return configs, fmt.Errorf("provisioning machine pool quantity must be positive")
		}

		node := pool.MachinePoolConfig.NodeConfig.Harvester
		if node == nil && template.NodeConfig != nil {
			node = template.NodeConfig.Harvester
		}

		if node == nil || len(node.Disks) == 0 || node.ImageName == "" {
			return configs, fmt.Errorf("Harvester machine pool needs a node configuration with an image and disk")
		}

		if err := node.Validate(); err != nil {
			return configs, err
		}

		configs.HarvesterMachineConfigs.VMNamespace = node.Namespace

		var roles []string
		if pool.MachinePoolConfig.ControlPlane {
			roles = append(roles, "controlplane")
		}

		if pool.MachinePoolConfig.Etcd {
			roles = append(roles, "etcd")
		}

		if pool.MachinePoolConfig.Worker {
			roles = append(roles, "worker")
		}

		if len(roles) == 0 {
			return configs, fmt.Errorf("provisioning machine pool needs at least one node role")
		}

		configs.HarvesterMachineConfigs.HarvesterMachineConfig = append(configs.HarvesterMachineConfigs.HarvesterMachineConfig, machinepools.HarvesterMachineConfig{
			Roles:    machinepools.Roles{Roles: roles},
			CPUCount: strconv.Itoa(node.CPU), MemorySize: strconv.Itoa(node.Memory),
			DiskSize: strconv.Itoa(node.Disks[0].Size), DiskBus: node.Disks[0].Bus,
			ImageName:   strings.Trim(node.ImageNamespace+"/"+node.ImageName, "/"),
			NetworkName: strings.Trim(network.Namespace+"/"+network.Name, "/"), SSHUser: sshUser,
		})
	}

	if len(configs.HarvesterMachineConfigs.HarvesterMachineConfig) == 0 {
		return configs, fmt.Errorf("provisioning template needs at least one machine pool")
	}

	return configs, nil
}

func ensureHarvesterCredential(ctx context.Context, recipe *dart.Dart, client *rancher.Client, clusterName string) (*v1.SteveAPIObject, error) {
	name := clusterName + "-credential"
	resource := client.Steve.SteveType("secret")

	existing, err := resource.ByID("cattle-global-data/" + name)
	if err == nil || !isSteveNotFound(err) {
		return existing, err
	}

	provisioningClient, err := client.GetKubeAPIProvisioningClient()
	if err != nil {
		return nil, err
	}

	harvesters, err := provisioningClient.Clusters(fleetNamespace).List(ctx, metav1.ListOptions{LabelSelector: "provider.cattle.io=harvester"})
	if err != nil {
		return nil, err
	}

	if len(harvesters.Items) != 1 || harvesters.Items[0].Status.ClusterName == "" {
		return nil, fmt.Errorf("expected exactly one imported Harvester cluster with a management ID")
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath, _ = recipe.TofuVariables["kubeconfig"].(string)

	rawConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{}).RawConfig()
	if err != nil {
		return nil, err
	}

	if err := clientcmdapi.MinifyConfig(&rawConfig); err != nil {
		return nil, err
	}

	if err := clientcmdapi.FlattenConfig(&rawConfig); err != nil {
		return nil, err
	}

	kubeconfig, err := clientcmd.Write(rawConfig)
	if err != nil {
		return nil, err
	}

	return resource.Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "cattle-global-data",
			Annotations: map[string]string{"provisioning.cattle.io/driver": "harvester", "field.cattle.io/name": name},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"harvestercredentialConfig-clusterId":         []byte(harvesters.Items[0].Status.ClusterName),
			"harvestercredentialConfig-clusterType":       []byte("imported"),
			"harvestercredentialConfig-kubeconfigContent": kubeconfig,
		},
	})
}

func executeRegistrationCommand(ctx context.Context, node shepherdnodes.Node, command string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	signer, err := ssh.ParsePrivateKey(node.SSHKey)
	if err != nil {
		return err
	}

	address := net.JoinHostPort(node.PublicIPAddress, "22")

	connection, err := (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer connection.Close()

	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stop()

	deadline := time.Now().Add(30 * time.Second)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}

	if err := connection.SetDeadline(deadline); err != nil {
		return err
	}

	sshConnection, channels, requests, err := ssh.NewClientConn(connection, address, &ssh.ClientConfig{
		User: node.SSHUser, Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return err
	}

	client := ssh.NewClient(sshConnection, channels, requests)
	defer client.Close()

	commandDeadline, _ := ctx.Deadline()
	if err := connection.SetDeadline(commandDeadline); err != nil {
		return err
	}

	sshSession, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sshSession.Close()

	return sshSession.Run(command)
}
