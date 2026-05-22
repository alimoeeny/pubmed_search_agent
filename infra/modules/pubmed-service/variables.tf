variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region for Cloud Run and Artifact Registry"
  type        = string
  default     = "us-east1"
}

variable "github_repo" {
  description = "GitHub repo in owner/name format, e.g. alimoeeny/pubmed_search"
  type        = string
}

variable "image_tag" {
  description = "Docker image tag to deploy (overridden by CI with git SHA)"
  type        = string
  default     = "latest"
}

variable "secrets" {
  description = "Map of secret names to their values (never committed)"
  type        = map(string)
  sensitive   = true
  # Expected keys: GOOGLE_API_KEY, NCBI_EMAIL, SUPABASE_JWT_SECRET, SUPABASE_DB_URL
}
