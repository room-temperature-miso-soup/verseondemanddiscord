terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 4.0"
    }
  }
}



resource "google_secret_manager_secret" "discord_token" {
  secret_id = "discord-token"
  
  replication {
    auto = true
  }
}

resource "google_secret_manager_secret_version" "discord_token_value" {
  secret = google_secret_manager_secret.discord_token.id
  secret_data = var.discord_token
}

resource "google_cloud_run_service" "discord_bot" {
  name     = "verseondemanddiscord"
  location = var.region

  template {
    spec {
      containers {
        image = "gcr.io/${var.project_id}/verseondemanddiscord:latest"
        env {
          name = "DISCORD_TOKEN"
          value_from {
            secret_key_ref {
              name = google_secret_manager_secret.discord_token.secret_id
              key  = "latest"
            }
          }
        }
      }
    }
  }
}
