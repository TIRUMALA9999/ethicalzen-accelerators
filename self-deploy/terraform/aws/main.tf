terraform {
  required_version = ">= 1.3.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# NOTE: This is a starter template.
# Create a VPC + EKS cluster (private preferred).

provider "aws" {
  region = var.region
}

variable "region" { type = string default = "us-east-1" }
variable "cluster_name" { type = string default = "ethicalzen-runtime" }

output "note" {
  value = "AWS template stub: add VPC + EKS resources and kubeconfig output"
}
