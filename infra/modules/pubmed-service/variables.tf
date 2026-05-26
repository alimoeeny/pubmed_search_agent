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

variable "cors_allowed_origins" {
  description = "Comma-separated list of allowed CORS origins for the API (e.g. your Cloudflare Pages URL). Empty = reflect any origin (dev only)."
  type        = string
  default     = ""
}

variable "secrets" {
  description = "Map of secret names to their values (never committed)"
  type        = map(string)
  sensitive   = true
  # Expected keys: GOOGLE_API_KEY, NCBI_EMAIL, SUPABASE_JWT_SECRET, SUPABASE_DB_URL
}
