output "first_server_private_name" {
  value = var.server_count > 0 ? module.server_nodes[0].private_name : null
}

output "first_server_public_name" {
  value = var.server_count > 0 ? module.server_nodes[0].public_name : null
}

output "kubeconfig" {
  value = module.rke2.kubeconfig
}

output "local_kubernetes_api_url" {
  value = module.rke2.local_kubernetes_api_url
}

output "context" {
  value = module.rke2.context
}

output "tunnel_app_http_port" {
  value = var.tunnel_app_http_port
}

output "tunnel_app_https_port" {
  value = var.tunnel_app_https_port
}

output "node_access_commands" {
  value = merge({
    for node in module.server_nodes : node.name => node.ssh_script_filename
    }, {
    for node in module.agent_nodes : node.name => node.ssh_script_filename
  })
}

output "ingress_class_name" {
  value = module.rke2.ingress_class_name
}
