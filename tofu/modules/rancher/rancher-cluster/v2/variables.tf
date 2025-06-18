variable "k8s_distribution" {
  type        = string
  default     = null
  description = "The K8s distribution to use for setting up the cluster. One of k3s, rke1, or rke2."
  validation {
    condition     = var.k8s_distribution == null ? true : contains(["k3s", "rke1", "rke2"], var.k8s_distribution)
    error_message = "Please pass in a string equal to one of the following: [\"k3s\", \"rke1\", \"rke2\"] or null."
  }
}

variable "k8s_version" {
  type        = string
  default     = null
  description = "Version of k8s to use for downstream cluster (RKE1 version string)"
}

variable "is_custom_cluster" {
  type        = bool
  default     = false
  description = "Flag that defines if cluster is a custom cluster. Ensures no `<distro>_config` block is created if true"
}

variable "roles_per_pool" {
  type = list(object({
    quantity      = number
    etcd          = optional(bool)
    control-plane = optional(bool)
    worker        = optional(bool)
  }))
  default = [{
    quantity      = 1
    control-plane = true
    etcd          = true
    worker        = true
  }]
  description = <<EOF
  A list of maps where each element contains keys that define the roles and quantity for a given node pool.
  Example: [
    {
      quantity = 3
      etcd = true
      control-plane = true
      worker = true
    }
  ]
  EOF
  validation {
    condition     = contains([1, 3, 5], sum([for i, pool in var.roles_per_pool : pool.quantity if try(pool.etcd == true, false)]))
    error_message = "The number of etcd nodes per cluster must be one of [1, 3, 5]."
  }
  validation {
    condition     = sum([for i, pool in var.roles_per_pool : pool.quantity if try(pool.control-plane == true, false)]) >= 1
    error_message = "The number of control-plane nodes per cluster must be >= 1."
  }
  validation {
    condition     = sum([for i, pool in var.roles_per_pool : pool.quantity if try(pool.worker == true, false)]) >= 1
    error_message = "The number of worker nodes per cluster must be >= 1."
  }
}

variable "name" {
  type        = string
  default     = "load-testing"
  description = "Unique identifier appended to the Rancher url subdomain"
}

variable "description" {
  type        = string
  default     = null
  description = "(optional) describe your variable"
}

variable "annotations" {
  type        = map(any)
  default     = null
  description = "Optional annotations for the Cluster"
}

variable "labels" {
  type        = map(any)
  default     = {}
  description = "Labels to add to each provisioned cluster"
}

variable "agent_env_vars" {
  type        = list(map(string))
  default     = null
  description = "List of maps for optional Agent Env Vars for Rancher agent. Just for Rancher v2.5.6 and above"
}

variable "sensitive_output" {
  type        = bool
  default     = false
  description = "Bool that determines if certain outputs should be marked as sensitive and be masked. Default: false"
}

variable "upgrade_strategy" {
  type        = any
  default     = null
  description = "(Optional/Computed) Upgrade strategy options for the proper cluster type (object with optional attributes for those defined here https://registry.terraform.io/providers/rancher/rancher2/latest/docs/resources/cluster)"
}

variable "network_config" {
  type        = any
  default     = null
  description = "(Optional/Computed) Network config options for any valid cluster config (object with optional attributes for any network-related options defined here https://registry.terraform.io/providers/rancher/rancher2/latest/docs/resources/cluster)"
}

variable "default_pod_security_admission_configuration_template_name" {
  type        = string
  default     = null
  description = "A string specifying which default Pod Security Admission Configuration Template (PSACT) to use"
  validation {
    condition     = var.default_pod_security_admission_configuration_template_name == null ? true : length(var.default_pod_security_admission_configuration_template_name) > 0 || contains([null, "", "rancher-privileged", "rancher-restricted"], var.default_pod_security_admission_configuration_template_name)
    error_message = "var.default_pod_security_admission_configuration_template_name must be one of [null, '','rancher-privileged', 'rancher-restricted'] OR the name of an existing PSACT."
  }
}

variable "fleet_agent_deployment_customization" {
  type = list(object({
    append_tolerations             = optional(list(map(string)), [])
    override_affinity              = optional(string, null)
    override_resource_requirements = optional(list(map(string)), [])
  }))
  default     = null
  description = "Optional customization for fleet agent. For Rancher v2.7.5 and above"
}

variable "cluster_auth_endpoint" {
  type = object({
    enabled  = bool
    fqdn     = string
    ca_certs = string
  })
  default = {
    enabled  = true
    fqdn     = null
    ca_certs = null
  }
  description = "Enabling the local cluster authorized endpoint allows direct communication with the cluster, bypassing the Rancher API proxy"
}
