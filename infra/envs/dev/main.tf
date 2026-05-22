terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = "us-east1"
}

# Dev environment — variables wired, not applied yet.
# Run `terraform apply` here when a dev GCP project is provisioned.
module "pubmed" {
  source      = "../../modules/pubmed-service"
  project_id  = var.project_id
  region      = "us-east1"
  github_repo = var.github_repo
  image_tag   = var.image_tag
  secrets     = var.secrets
}
