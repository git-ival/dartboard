terraform {
  required_providers {
    ssh = {
      source = "loafoe/ssh"
    }
  }
}

module "server_node" {
  source                = "../node"
  project_name          = var.project_name
  name                  = "${var.name}-server"
  ssh_private_key_path  = var.ssh_private_key_path
  node_module           = var.node_module
  node_module_variables = var.node_module_variables
  network_config        = var.network_config
}

resource "ssh_resource" "install_postgres" {
  depends_on = [module.server_node]

  host         = module.server_node.private_name
  private_key  = file(var.ssh_private_key_path)
  user         = var.ssh_user
  bastion_host = var.network_config.ssh_bastion_host
  bastion_user = var.network_config.ssh_user

  file {
    source      = var.kine_executable != null ? var.kine_executable : "/dev/null"
    destination = "/tmp/kine"
    permissions = "0755"
  }

  file {
    content = templatefile("${path.module}/install_postgres.sh", {
      gogc         = tostring(var.gogc)
      kine_version = var.kine_version
    })
    destination = "/root/install_postgres.sh"
    permissions = "0700"
  }

  commands = [
    "/root/install_postgres.sh > >(tee install_postgres.log) 2> >(tee install_postgres.err >&2)"
  ]
}
