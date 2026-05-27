terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
  # State is local; .tfstate is git-ignored.
  # Migrate to a GCS backend with:
  #   backend "gcs" { bucket = "<state-bucket>"; prefix = "prod" }
}

provider "google" {
  project = var.project_id
  region  = "us-east1"
}

module "pubmed" {
  source               = "../../modules/pubmed-service"
  project_id           = var.project_id
  region               = "us-east1"
  github_repo          = var.github_repo
  image_tag            = var.image_tag
  supabase_url         = var.supabase_url
  cors_allowed_origins = var.cors_allowed_origins
  secrets              = var.secrets
}

output "workload_identity_provider" {
  value = module.pubmed.workload_identity_provider
}

output "github_actions_sa_email" {
  value = module.pubmed.github_actions_sa_email
}

output "artifact_registry_url" {
  value = module.pubmed.artifact_registry_url
}

output "cloud_run_url" {
  value = module.pubmed.cloud_run_url
}
