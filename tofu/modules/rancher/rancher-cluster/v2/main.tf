resource "rancher2_cluster_v2" "rke2" {
  name                                                       = var.name
  labels                                                     = var.labels
  kubernetes_version                                         = var.k8s_version
  default_pod_security_admission_configuration_template_name = var.default_pod_security_admission_configuration_template_name

  rke_config {
    dynamic "machine_pools" {
      for_each = var.roles_per_pool
      iterator = pool
      content {
        name                         = "${var.name}${pool.key}"
        cloud_credential_secret_name = data.rancher2_cloud_credential.this.id
        control_plane_role           = try(tobool(pool.value["control-plane"]), false)
        worker_role                  = try(tobool(pool.value["worker"]), false)
        etcd_role                    = try(tobool(pool.value["etcd"]), false)
        quantity                     = try(tonumber(pool.value["quantity"]), 1)

        machine_config {
          kind = rancher2_machine_config_v2.aws.kind
          name = rancher2_machine_config_v2.aws.name
        }
      }
    }
  }

  depends_on = [
    data.rancher2_cloud_credential.this
  ]
}
