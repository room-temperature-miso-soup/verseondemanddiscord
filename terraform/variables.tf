variable "project_id" {
  description = "GCP Project ID"
  type        = string
}

variable "region" {
  description = "GCP Region"
  type        = string
  default     = "us-central1"
}

variable "discord_token" {
  description = "Discord Bot Token"
  type        = string
  sensitive   = true
}
