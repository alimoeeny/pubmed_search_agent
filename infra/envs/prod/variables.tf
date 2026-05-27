variable "project_id" {
  type = string
}

variable "github_repo" {
  type = string
}

variable "image_tag" {
  type    = string
  default = "latest"
}

variable "supabase_url" {
  description = "Supabase project URL, e.g. https://<ref>.supabase.co"
  type        = string
}

variable "cors_allowed_origins" {
  type    = string
  default = ""
}

variable "secrets" {
  type      = map(string)
  sensitive = true
}
