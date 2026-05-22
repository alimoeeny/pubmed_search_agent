output "workload_identity_provider" {
  description = "WIF provider resource name — set as WIF_PROVIDER GitHub secret"
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "github_actions_sa_email" {
  description = "Deployer service account email — set as WIF_SERVICE_ACCOUNT GitHub secret"
  value       = google_service_account.github_actions.email
}

output "artifact_registry_url" {
  description = "Docker registry URL prefix for image tags"
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.pubmed.repository_id}"
}

output "cloud_run_url" {
  description = "Public URL of the deployed Cloud Run service"
  value       = google_cloud_run_v2_service.agent.uri
}
