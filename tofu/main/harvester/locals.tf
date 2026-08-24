locals {
  node_module_variables = concat(
    var.upstream_cluster != null ? [var.upstream_cluster.node_module_variables] : [],
    var.tester_cluster != null ? [var.tester_cluster.node_module_variables] : [],
    [for template in var.downstream_cluster_templates : template.node_module_variables],
  )
  image_names = toset(compact([for config in local.node_module_variables : try(config.image_name, null) != null && try(config.image_id, null) == null ? "${coalesce(try(config.image_namespace, null), var.namespace)}/${config.image_name}" : null]))
  ssh_key_names = toset(flatten([for config in local.node_module_variables : [for key in coalesce(try(config.ssh_shared_public_keys, null), []) : "${key.namespace}/${key.name}"]]))
  images_by_name = { for key, image in data.harvester_image.images_by_name : key => image.id }
  ssh_keys_by_name = { for key, ssh_key in data.harvester_ssh_key.ssh_keys : key => { id = ssh_key.id, public_key = ssh_key.public_key } }
}
data "harvester_image" "images_by_name" { for_each = local.image_names namespace = split("/", each.value)[0] display_name = split("/", each.value)[1] }
data "harvester_ssh_key" "ssh_keys" { for_each = local.ssh_key_names namespace = split("/", each.value)[0] name = split("/", each.value)[1] }
