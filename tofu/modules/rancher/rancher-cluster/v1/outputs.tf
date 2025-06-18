output "id" {
  value = rancher2_cluster.this.id
}

output "name" {
  value = rancher2_cluster.this.name
}

output "default_project_id" {
  value = rancher2_cluster.this.default_project_id
}

output "cluster_registration_token" {
  value = rancher2_cluster.this.cluster_registration_token
}

output "registration_command" {
  value = rancher2_cluster.this.cluster_registration_token[0].command
}

output "insecure_registration_command" {
  value = rancher2_cluster.this.cluster_registration_token[0].insecure_command
}

output "driver" {
  value = rancher2_cluster.this.driver
}

output "kube_config" {
  value     = var.sensitive_output ? rancher2_cluster.this.kube_config : nonsensitive(rancher2_cluster.this.kube_config)
  sensitive = true
}
