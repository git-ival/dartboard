# Get imported harvester cluster info
resource "rancher2_cluster_v2" "this" {
  name = var.harvester_name
  kubernetes_version = var.harvester_k8s_version
}

resource "ssh_resource" "additional_server_installation" {
  host         = module.server_nodes[count.index + 1].private_name
  private_key  = file(var.ssh_private_key_path)
  user         = var.ssh_user
  bastion_host = var.network_config.ssh_bastion_host
  bastion_user = var.network_config.ssh_bastion_user
  timeout      = "600s"

  commands = [
    rancher2_cluster_v2.this.cluster_registration_token
  ]
}

# Get imported harvester cluster info
data "rancher2_cluster_v2" "this" {
  name = var.harvester_name

  depends_on = [  ]
}

# Create a new cloud credential for an imported Harvester cluster
resource "rancher2_cloud_credential" "this" {
  name = "${var.harvester_name}-cred"
  harvester_credential_config {
    cluster_id = data.rancher2_cluster_v2.this.cluster_v1_id
    cluster_type = "imported"
    kubeconfig_content = data.rancher2_cluster_v2.this.kube_config
  }

}
