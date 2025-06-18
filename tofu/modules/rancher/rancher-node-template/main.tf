resource "rancher2_node_template" "this" {
  count                    = var.create_new ? 1 : 0
  name                     = var.name
  cloud_credential_id      = var.cloud_cred_id
  engine_env               = try(var.engine_fields.engine_env, null)
  engine_insecure_registry = try(var.engine_fields.engine_insecure_registry, null)
  engine_install_url       = try(var.engine_fields.engine_install_url == null && length(var.install_docker_version) > 0 ? "https://releases.rancher.com/install-docker/${var.install_docker_version}.sh" : var.engine_fields.engine_install_url, null)
  engine_label             = try(var.engine_fields.engine_label, null)
  engine_opt               = try(var.engine_fields.engine_opt, null)
  engine_registry_mirror   = try(var.engine_fields.engine_registry_mirror, null)
  engine_storage_driver    = try(var.engine_fields.engine_storage_driver, null)
  dynamic "node_taints" {
    for_each = try(var.node_taints, [])
    iterator = taint
    content {
      key        = try(taint.value.key, null)
      value      = try(taint.value.value, null)
      effect     = try(taint.value.effect, null)
      time_added = try(taint.value.time_added, null)
    }
  }
  use_internal_ip_address = try(var.use_internal_ip_address, null)
  annotations             = try(var.annotations, null)
  labels                  = try(var.labels, null)

  dynamic "amazonec2_config" {
    for_each = var.cloud_provider == "aws" ? [1] : []
    content {
      ami                        = try(var.node_config.ami, null)
      region                     = try(var.node_config.region, null)
      security_group             = try(var.node_config.security_group, null)
      subnet_id                  = try(var.node_config.subnet_id, null)
      vpc_id                     = try(var.node_config.vpc_id, null)
      zone                       = try(var.node_config.zone, null)
      access_key                 = try(var.node_config.access_key, null)
      block_duration_minutes     = try(var.node_config.block_duration_minutes, null)
      device_name                = try(var.node_config.device_name, null)
      encrypt_ebs_volume         = try(var.node_config.encrypt_ebs_volume, null)
      endpoint                   = try(var.node_config.endpoint, null)
      http_endpoint              = try(var.node_config.http_endpoint, null)
      http_tokens                = try(var.node_config.http_tokens, null)
      iam_instance_profile       = try(var.node_config.iam_instance_profile, null)
      insecure_transport         = try(var.node_config.insecure_transport, null)
      instance_type              = try(var.node_config.instance_type, null)
      kms_key                    = try(var.node_config.kms_key, null)
      monitoring                 = try(var.node_config.monitoring, null)
      open_port                  = try(var.node_config.open_port, null)
      private_address_only       = try(var.node_config.private_address_only, null)
      request_spot_instance      = try(var.node_config.request_spot_instance, null)
      retries                    = try(var.node_config.retries, null)
      root_size                  = try(var.node_config.root_size, null)
      secret_key                 = try(var.node_config.secret_key, null)
      security_group_readonly    = try(var.node_config.security_group_readonly, null)
      session_token              = try(var.node_config.session_token, null)
      spot_price                 = try(var.node_config.spot_price, null)
      ssh_user                   = try(var.node_config.ssh_user, null)
      tags                       = try(var.node_config.tags, null)
      use_ebs_optimized_instance = try(var.node_config.use_ebs_optimized_instance, null)
      use_private_address        = try(var.node_config.private_ip_address, null)
      userdata                   = try(var.node_config.userdata, null)
      volume_type                = try(var.node_config.volume_type, null)
    }
  }

  dynamic "linode_config" {
    for_each = var.cloud_provider == "linode" ? [1] : []
    content {
      authorized_users  = try(var.node_config.authorized_users, null)
      create_private_ip = try(var.node_config.create_private_ip, null)
      docker_port       = try(var.node_config.docker_port, null)
      image             = try(var.node_config.image, null)
      instance_type     = try(var.node_config.instance_type, null)
      label             = try(var.node_config.label, null)
      region            = try(var.node_config.region, null)
      root_pass         = try(var.node_config.root_pass, null)
      ssh_port          = try(var.node_config.ssh_port, null)
      ssh_user          = try(var.node_config.ssh_user, null)
      stackscript       = try(var.node_config.stackscript, null)
      stackscript_data  = try(var.node_config.stackscript_data, null)
      swap_size         = try(var.node_config.swap_size, null)
      tags              = try(var.node_config.tags, null)
      token             = try(var.node_config.token, null)
      ua_prefix         = try(var.node_config.ua_prefix, null)
    }
  }

  dynamic "harvester_config" {
    for_each = var.cloud_provider == "harvester" ? [1] : []
    content {
      vm_namespace = try(var.node_config.vm_namespace, null)
      cpu_count = try(var.node_config.cpu_count, null)
      memory_size = try(var.node_config.memory_size, null)
      disk_info = try(var.node_config.disk_info, null)
      ssh_user = try(var.node_config.ssh_user, null)
      ssh_password = try(var.node_config.ssh_password, null)
      network_info = try(var.node_config.network_info, null)
      user_data = try(var.node_config.user_data, null)
      network_data = try(var.node_config.network_data, null)
      vm_affinity = try(var.node_config.vm_affinity, null)
    }
  }
}

### Only create a node_template if the caller has defined that a new node_template should be created
### else, look for an existing node_template with the given name
data "rancher2_node_template" "this" {
  name = var.create_new ? rancher2_node_template.this[0].name : var.name
}
