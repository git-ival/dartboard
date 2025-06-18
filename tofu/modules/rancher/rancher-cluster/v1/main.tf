terraform {
  required_version = ">= 0.13"
  required_providers {
    rancher2 = {
      source = "rancher/rancher2"
    }
  }
}

resource "rancher2_cluster" "this" {
  name                                                       = var.name
  description                                                = try(var.description, null)
  labels                                                     = try(var.labels, null)
  annotations                                                = try(var.annotations, null)
  default_pod_security_admission_configuration_template_name = try(var.default_pod_security_admission_configuration_template_name, null)
  cluster_auth_endpoint {
    enabled  = try(var.cluster_auth_endpoint.enabled, null)
    fqdn     = try(var.cluster_auth_endpoint.fqdn, null)
    ca_certs = try(var.cluster_auth_endpoint.ca_certs, null)
  }
  dynamic "fleet_agent_deployment_customization" {
    for_each = var.fleet_agent_deployment_customization != null ? var.fleet_agent_deployment_customization : []
    iterator = customization
    content {
      dynamic "append_tolerations" {
        for_each = customization.value.append_tolerations
        iterator = toleration
        content {
          key      = try(toleration.value.key, "")
          operator = try(toleration.value.operator, "")
          value    = try(toleration.value.value, "")
          effect   = try(toleration.value.effect, "")
          seconds  = try(toleration.value.seconds, null)
        }
      }
      override_affinity = customization.value.override_affinity
      dynamic "override_resource_requirements" {
        for_each = customization.value.override_resource_requirements
        iterator = requirement
        content {
          cpu_limit      = try(requirement.value.cpu_limit, "")
          cpu_request    = try(requirement.value.cpu_request, "")
          memory_limit   = try(requirement.value.memory_limit, "")
          memory_request = try(requirement.value.memory_request, "")
        }
      }
    }
  }
  dynamic "agent_env_vars" {
    for_each = var.agent_env_vars != null ? var.agent_env_vars : []
    iterator = env_var
    content {
      name  = env_var.value.name
      value = env_var.value.value
    }
  }

  # If provisioning OR importing RKE1 cluster(s) use the following
  dynamic "rke_config" {
    for_each = var.k8s_distribution == "rke1" && !var.is_custom_cluster ? [1] : []

    content {
      kubernetes_version    = var.k8s_version
      ignore_docker_version = false
      addons_include        = var.addons_include
      addons                = var.addons
      addon_job_timeout     = 60
      enable_cri_dockerd    = var.enable_cri_dockerd

      dynamic "cloud_provider" {
        for_each = var.cloud_provider != null ? [1] : []
        content {
          name                  = var.cloud_provider
          custom_cloud_provider = var.cloud_provider == "custom" ? var.cloud_config : null
        }
      }
      dynamic "network" {
        for_each = var.network_config != null ? [1] : []
        content {
          mtu     = try(var.network_config.mtu, null)
          options = try(var.network_config.options, null)
          plugin  = try(var.network_config.plugin, null)
          dynamic "tolerations" {
            for_each = try(var.network_config.tolerations != null, false) ? [var.network_config.tolerations] : []
            iterator = toleration
            content {
              key      = try(toleration.key, null)
              effect   = try(toleration.effect, null)
              operator = try(toleration.operator, null)
              seconds  = try(toleration.seconds, null)
              value    = try(toleration.value, null)
            }
          }
        }
      }
      services {
        dynamic "kube_api" {
          for_each = var.kube_api != null ? [var.kube_api] : []
          iterator = item
          content {
            # admission_configuration = try(item.value.admission_configuration, null)
            dynamic "admission_configuration" {
              for_each = try(item.value.admission_configuration != null, false) ? [item.value.admission_configuration] : []
              iterator = config
              content {
                api_version = try(config.value.api_version, null)
                kind        = try(config.value.kind, null)
                dynamic "plugins" {
                  for_each = config.value.plugins != null ? config.value.plugins : []
                  iterator = plugin
                  content {
                    name          = try(plugin.value.name, null)
                    path          = try(plugin.value.path, null)
                    configuration = try(plugin.value.configuration, null)
                  }
                }
              }
            }
            always_pull_images = try(item.value.always_pull_images, null)
            extra_args          = try(item.value.extra_args, null)
            extra_binds         = try(item.value.extra_binds, null)
            extra_env           = try(item.value.extra_env, null)
            image               = try(item.value.image, null)
            dynamic "secrets_encryption_config" {
              for_each = try(item.value.secrets_encryption_config != null, false) ? [item.value.secrets_encryption_config] : []
              content {
                enabled       = try(secrets_encryption_config.enabled, null)
                custom_config = try(secrets_encryption_config.custom_config, null)
              }
            }
            service_cluster_ip_range = try(item.value.service_cluster_ip_range, null)
            service_node_port_range  = try(item.value.service_node_port_range, null)
          }
        }
        dynamic "kubelet" {
          for_each = var.kubelet != null ? [var.kubelet] : []
          iterator = item
          content {
            cluster_dns_server           = try(item.value.cluster_dns_server, null)
            cluster_domain               = try(item.value.cluster_domain, null)
            extra_args                   = try(item.value.extra_args, null)
            extra_binds                  = try(item.value.extra_binds, null)
            extra_env                    = try(item.value.extra_env, null)
            fail_swap_on                 = try(item.value.fail_swap_on, null)
            generate_serving_certificate = try(item.value.generate_serving_certificate, null)
            image                        = try(item.value.image, null)
            infra_container_image        = try(item.value.infra_container_image, null)
          }
        }
        dynamic "kube_controller" {
          for_each = var.kube_controller != null ? [var.kube_controller] : []
          iterator = item
          content {
            cluster_cidr             = try(item.value.cluster_cidr, null)
            extra_args               = try(item.value.extra_args, null)
            extra_binds              = try(item.value.extra_binds, null)
            extra_env                = try(item.value.extra_env, null)
            image                    = try(item.value.image, null)
            service_cluster_ip_range = try(item.value.service_cluster_ip_range, null)
          }
        }
      }
      dynamic "upgrade_strategy" {
        for_each = var.upgrade_strategy != null ? [var.upgrade_strategy] : []
        iterator = item
        content {
          drain = try(item.value.drain, null)
          drain_input {
            delete_local_data  = try(item.value.drain_input.delete_local_data, null)
            force              = try(item.value.drain_input.force, null)
            grace_period       = try(item.value.drain_input.grace_period, null)
            ignore_daemon_sets = try(item.value.drain_input.ignore_daemon_sets, null)
            timeout            = try(item.value.drain_input.timeout, null)
          }
          max_unavailable_controlplane = try(item.value.max_unavailable_controlplane, null)
          max_unavailable_worker       = try(item.value.max_unavailable_worker, null)
        }
      }
    }
  }
  # If importing RKE2 cluster(s) use the following
  dynamic "rke2_config" {
    for_each = var.k8s_distribution == "rke2" && !var.is_custom_cluster ? [1] : []
    content {
      version = var.k8s_version
      dynamic "upgrade_strategy" {
        for_each = var.upgrade_strategy != null ? [var.upgrade_strategy] : []
        iterator = item
        content {
          drain_server_nodes = try(item.value.drain_server_nodes, null)
          drain_worker_nodes = try(item.value.drain_worker_nodes, null)
          server_concurrency = try(item.value.server_concurrency, null)
          worker_concurrency = try(item.value.worker_concurrency, null)
        }
      }
    }
  }

  # If importing K3s cluster(s) use the following
  dynamic "k3s_config" {
    for_each = var.k8s_distribution == "k3s" && !var.is_custom_cluster ? [1] : []
    content {
      version = var.k8s_version
      dynamic "upgrade_strategy" {
        for_each = var.upgrade_strategy != null ? [var.upgrade_strategy] : []
        iterator = item
        content {
          drain_server_nodes = try(item.value.drain_server_nodes, null)
          drain_worker_nodes = try(item.value.drain_worker_nodes, null)
          server_concurrency = try(item.value.server_concurrency, null)
          worker_concurrency = try(item.value.worker_concurrency, null)
        }
      }
    }
  }

}
