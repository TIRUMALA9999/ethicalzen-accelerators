# =============================================================================
# EthicalZen Runtime Enforcement - GCP Infrastructure
# Creates: VPC + Subnet + GKE cluster + Node pool
# =============================================================================

terraform {
  required_version = ">= 1.3.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

# =============================================================================
# Variables
# =============================================================================

variable "project_id" {
  description = "GCP Project ID"
  type        = string
}

variable "region" {
  description = "GCP Region"
  type        = string
  default     = "us-central1"
}

variable "cluster_name" {
  description = "GKE Cluster name"
  type        = string
  default     = "ethicalzen-runtime"
}

variable "network_name" {
  description = "VPC Network name"
  type        = string
  default     = "ethicalzen-vpc"
}

variable "subnet_cidr" {
  description = "Subnet CIDR range"
  type        = string
  default     = "10.10.0.0/24"
}

variable "pod_cidr" {
  description = "Pod CIDR range"
  type        = string
  default     = "10.20.0.0/16"
}

variable "service_cidr" {
  description = "Service CIDR range"
  type        = string
  default     = "10.30.0.0/16"
}

variable "node_count" {
  description = "Number of nodes in the node pool"
  type        = number
  default     = 2
}

variable "machine_type" {
  description = "Machine type for nodes"
  type        = string
  default     = "e2-standard-2"
}

# =============================================================================
# VPC Network
# =============================================================================

resource "google_compute_network" "vpc" {
  name                    = var.network_name
  auto_create_subnetworks = false
  description             = "VPC for EthicalZen Runtime Enforcement"
}

resource "google_compute_subnetwork" "subnet" {
  name          = "${var.network_name}-subnet"
  ip_cidr_range = var.subnet_cidr
  region        = var.region
  network       = google_compute_network.vpc.id

  secondary_ip_range {
    range_name    = "pods"
    ip_cidr_range = var.pod_cidr
  }

  secondary_ip_range {
    range_name    = "services"
    ip_cidr_range = var.service_cidr
  }

  private_ip_google_access = true
}

# =============================================================================
# Firewall Rules
# =============================================================================

resource "google_compute_firewall" "allow_internal" {
  name    = "${var.network_name}-allow-internal"
  network = google_compute_network.vpc.name

  allow {
    protocol = "tcp"
    ports    = ["0-65535"]
  }

  allow {
    protocol = "udp"
    ports    = ["0-65535"]
  }

  allow {
    protocol = "icmp"
  }

  source_ranges = [var.subnet_cidr, var.pod_cidr, var.service_cidr]
}

resource "google_compute_firewall" "allow_gateway" {
  name    = "${var.network_name}-allow-gateway"
  network = google_compute_network.vpc.name

  allow {
    protocol = "tcp"
    ports    = ["80", "443", "8443"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["ethicalzen-gateway"]
}

# =============================================================================
# GKE Cluster
# =============================================================================

resource "google_container_cluster" "primary" {
  name     = var.cluster_name
  location = var.region

  # We can't create a cluster with no node pool, so we create the smallest possible
  # and immediately delete it.
  remove_default_node_pool = true
  initial_node_count       = 1

  network    = google_compute_network.vpc.name
  subnetwork = google_compute_subnetwork.subnet.name

  ip_allocation_policy {
    cluster_secondary_range_name  = "pods"
    services_secondary_range_name = "services"
  }

  # Enable Workload Identity
  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  # Enable private cluster
  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = false
    master_ipv4_cidr_block  = "172.16.0.0/28"
  }

  # Master authorized networks
  master_authorized_networks_config {
    cidr_blocks {
      cidr_block   = "0.0.0.0/0"
      display_name = "All networks (adjust for production)"
    }
  }

  deletion_protection = false
}

resource "google_container_node_pool" "primary_nodes" {
  name       = "${var.cluster_name}-node-pool"
  location   = var.region
  cluster    = google_container_cluster.primary.name
  node_count = var.node_count

  node_config {
    machine_type = var.machine_type
    disk_size_gb = 50
    disk_type    = "pd-standard"

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]

    tags = ["ethicalzen-gateway", "ethicalzen-node"]

    workload_metadata_config {
      mode = "GKE_METADATA"
    }
  }

  autoscaling {
    min_node_count = 1
    max_node_count = 5
  }

  management {
    auto_repair  = true
    auto_upgrade = true
  }
}

# =============================================================================
# Cloud NAT (for private nodes to access internet)
# =============================================================================

resource "google_compute_router" "router" {
  name    = "${var.network_name}-router"
  region  = var.region
  network = google_compute_network.vpc.id
}

resource "google_compute_router_nat" "nat" {
  name                               = "${var.network_name}-nat"
  router                             = google_compute_router.router.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}

# =============================================================================
# Outputs
# =============================================================================

output "cluster_name" {
  value       = google_container_cluster.primary.name
  description = "GKE Cluster name"
}

output "cluster_endpoint" {
  value       = google_container_cluster.primary.endpoint
  description = "GKE Cluster endpoint"
  sensitive   = true
}

output "cluster_ca_certificate" {
  value       = base64decode(google_container_cluster.primary.master_auth[0].cluster_ca_certificate)
  description = "GKE Cluster CA certificate"
  sensitive   = true
}

output "get_credentials_command" {
  value       = "gcloud container clusters get-credentials ${google_container_cluster.primary.name} --region ${var.region} --project ${var.project_id}"
  description = "Command to get kubectl credentials"
}

output "helm_install_command" {
  value       = "helm upgrade --install ethicalzen-runtime ../../helm/ethicalzen-runtime -n ethicalzen-runtime --create-namespace -f ../../values/gcp.yaml"
  description = "Command to install Helm chart"
}
