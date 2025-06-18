resource "rancher2_cloud_credential" "this" {
  count = var.create_new ? 1 : 0
  name  = var.name

  dynamic "amazonec2_credential_config" {
    for_each = var.cloud_provider == "aws" ? [1] : []
    content {
      access_key     = var.credential_config.access_key
      secret_key     = var.credential_config.secret_key
      default_region = var.credential_config.region
    }
  }
  dynamic "linode_credential_config" {
    for_each = var.cloud_provider == "linode" ? [1] : []
    content {
      token = var.credential_config.token
    }
  }
  dynamic "harvester_credential_config " {
    for_each = var.cloud_provider == "harvester" ? [1] : []
    content {
      cluster_id         = var.credential_config.cluster_v1_id
      cluster_type       = var.credential_config.cluster_type
      kubeconfig_content = var.credential_config.kubeconfig_content
    }
  }
}

### Only create a new cloud_credential if the caller has defined that a new cloud_credential should be created
### else, look for an existing cloud_credential with the given name
data "rancher2_cloud_credential" "this" {
  name = var.create_new ? rancher2_cloud_credential.this[0].name : var.name
}
