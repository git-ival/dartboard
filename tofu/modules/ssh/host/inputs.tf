variable "project_name" {
  description = "A prefix for names of objects created by this module"
  default     = "st"
}

variable "name" {
  description = "Symbolic name of this host"
  type        = string
}

variable "fqdn" {
  description = "Host fully-qualified domain name for SSH access"
  type        = string
}

variable "ssh_user" {
  description = "User name for SSH access"
  type        = string
}

variable "ssh_private_key_path" {
  description = "Path of private ssh key used to access the instance"
  type        = string
}

variable "host_configuration_commands" {
  description = "Commands to run when the host is deployed"
  default     = ["cat /etc/os-release"]
}
