terraform {
  required_version = ">= 1.3.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
  }
}

# NOTE: This is a starter template.
# Create a VNet + AKS cluster (private preferred).

provider "azurerm" {
  features {}
}

variable "location" { type = string default = "eastus" }
variable "cluster_name" { type = string default = "ethicalzen-runtime" }

output "note" {
  value = "Azure template stub: add VNet + AKS resources and kubeconfig output"
}
