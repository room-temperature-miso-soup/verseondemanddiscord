output "service_url" {
  value = google_cloud_run_service.discord_bot.status[0].url
}
