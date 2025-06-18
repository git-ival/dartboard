# node-template

This component module can be used to create or retrieve a `rancher2_machine_config_v2` resource.

<!-- BEGINNING OF PRE-COMMIT-TERRAFORM DOCS HOOK -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](https://developer.hashicorp.com/terraform/install) or [opentofu](https://opentofu.org/docs/intro/install/) | >= 1.1.0 |
| <a name="requirement_rancher2"></a> [rancher2](https://registry.terraform.io/providers/rancher/rancher2/) | >= 1.10.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_rancher2"></a> [rancher2](https://registry.terraform.io/providers/rancher/rancher2/) | 1.23.0 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [rancher2_machine_config_v2.this](https://registry.terraform.io/providers/rancher/rancher2/latest/docs/resources/node_template) | resource |
| [rancher2_machine_config_v2.this](https://registry.terraform.io/providers/rancher/rancher2/latest/docs/data-sources/node_template) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_annotations"></a> [annotations](#input\_annotations) | Annotations for Node Template | `map(string)` | `null` | no |
| <a name="input_cloud_provider"></a> [cloud\_provider](#input\_cloud\_provider) | n/a | `string` | n/a | yes |
| <a name="input_create_new"></a> [create\_new](#input\_create\_new) | Flag defining if a new node template should be created on each tf apply. Useful for scripting purposes | `bool` | `true` | no |
| <a name="input_labels"></a> [labels](#input\_labels) | Labels for Node Template | `map(string)` | `null` | no |
| <a name="input_name"></a> [generate_name](#input\_generate\_name) | n/a | `string` | n/a | yes |
| <a name="input_node_config"></a> [node\_config](#input\_node\_config) | (Optional/Computed) Cloud provider-specific configuration object (object with optional attributes for those defined here https://registry.terraform.io/providers/rancher/rancher2/7.0.0/docs/resources/node_template#argument-reference) | `any` | n/a | yes |
| <a name="input_node_taints"></a> [node\_taints](#input\_node\_taints) | Node taints. For Rancher v2.3.3 or above | <pre>list(object({<br>    key        = optional(string, null)<br>    value      = optional(string, null)<br>    effect     = optional(string, null)<br>    time_added = optional(string, null)<br>  }))</pre> | `[]` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_id"></a> [id](#output\_id) | n/a |
| <a name="output_name"></a> [name](#output\_name) | n/a |
| <a name="output_node_template"></a> [node\_template](#output\_node\_template) | n/a |
<!-- END OF PRE-COMMIT-TERRAFORM DOCS HOOK -->
