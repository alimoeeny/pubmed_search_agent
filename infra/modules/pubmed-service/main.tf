terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

locals {
  service_name = "pubmed-agent"
  secret_names = ["SUPABASE_DB_URL"]
}

# ── Artifact Registry ─────────────────────────────────────────────────────────

resource "google_artifact_registry_repository" "pubmed" {
  project       = var.project_id
  location      = var.region
  repository_id = local.service_name
  format        = "DOCKER"
  description   = "Docker images for the PubMed agent"
}

# ── GCS — PDF storage ─────────────────────────────────────────────────────────

resource "google_storage_bucket" "pdfs" {
  project                     = var.project_id
  name                        = "${var.project_id}-pubmed-pdfs"
  location                    = var.region
  uniform_bucket_level_access = true
  force_destroy               = false
}

# ── Service accounts ──────────────────────────────────────────────────────────

resource "google_service_account" "cloud_run" {
  project      = var.project_id
  account_id   = "${local.service_name}-run"
  display_name = "PubMed Agent — Cloud Run runtime"
}

resource "google_service_account" "github_actions" {
  project      = var.project_id
  account_id   = "${local.service_name}-deploy"
  display_name = "PubMed Agent — GitHub Actions deployer"
}

# ── IAM: Cloud Run SA → GCS ───────────────────────────────────────────────────

resource "google_storage_bucket_iam_member" "run_pdf_admin" {
  bucket = google_storage_bucket.pdfs.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.cloud_run.email}"
}

# ── Secret Manager ────────────────────────────────────────────────────────────

resource "google_secret_manager_secret" "app_secrets" {
  for_each  = toset(local.secret_names)
  project   = var.project_id
  secret_id = each.key

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "app_secrets" {
  for_each    = toset(local.secret_names)
  secret      = google_secret_manager_secret.app_secrets[each.key].id
  secret_data = var.secrets[each.key]
}

resource "google_secret_manager_secret_iam_member" "run_access" {
  for_each  = toset(local.secret_names)
  project   = var.project_id
  secret_id = google_secret_manager_secret.app_secrets[each.key].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.cloud_run.email}"
}

# ── Cloud Run v2 service ──────────────────────────────────────────────────────

resource "google_cloud_run_v2_service" "agent" {
  project  = var.project_id
  name     = local.service_name
  location = var.region

  template {
    service_account = google_service_account.cloud_run.email

    timeout = "3600s"

    containers {
      image = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.pubmed.repository_id}/${local.service_name}:${var.image_tag}"

      resources {
        limits = {
          cpu    = "2"
          memory = "2Gi"
        }
        startup_cpu_boost = true
      }

      dynamic "env" {
        for_each = toset(local.secret_names)
        content {
          name = env.key
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.app_secrets[env.key].secret_id
              version = "latest"
            }
          }
        }
      }

      env {
        name  = "PDF_GCS_BUCKET"
        value = google_storage_bucket.pdfs.name
      }

      env {
        name  = "SERVER"
        value = "true"
      }

      env {
        name  = "SUPABASE_URL"
        value = var.supabase_url
      }

      env {
        name  = "CORS_ALLOWED_ORIGINS"
        value = var.cors_allowed_origins
      }
    }

    scaling {
      min_instance_count = 0
      max_instance_count = 10
    }
  }

  depends_on = [google_secret_manager_secret_version.app_secrets]
}

# ── IAM: Cloud Run SA → Vertex AI ────────────────────────────────────────────
# Grants the runtime service account permission to call Vertex AI (Gemini).
# No API key is needed — the SDK uses ADC with this service account identity.

resource "google_project_iam_member" "run_vertex_user" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${google_service_account.cloud_run.email}"
}

# ── Public invoker (JWT is our auth layer) ────────────────────────────────────

resource "google_cloud_run_v2_service_iam_member" "public_invoker" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.agent.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

# ── Workload Identity Federation for GitHub Actions ───────────────────────────

resource "google_iam_workload_identity_pool" "github" {
  project                   = var.project_id
  workload_identity_pool_id = "github-pool"
  display_name              = "GitHub Actions pool"
}

resource "google_iam_workload_identity_pool_provider" "github" {
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-provider"
  display_name                       = "GitHub OIDC provider"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
  }

  attribute_condition = "assertion.repository == '${var.github_repo}'"
}

# ── IAM: GitHub Actions SA ────────────────────────────────────────────────────

resource "google_service_account_iam_member" "github_wif" {
  service_account_id = google_service_account.github_actions.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/${var.github_repo}"
}

resource "google_project_iam_member" "github_ar_writer" {
  project = var.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_project_iam_member" "github_run_admin" {
  project = var.project_id
  role    = "roles/run.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_service_account_iam_member" "github_run_sa_user" {
  service_account_id = google_service_account.cloud_run.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.github_actions.email}"
}
