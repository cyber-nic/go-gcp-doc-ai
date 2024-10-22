
resource "google_cloudfunctions2_function" "nlp_worker" {
  name        = "nlp-worker"
  location   = local.region

   build_config {
    runtime = "go121"
    entry_point = "handler"
    source {
      repo_source {
        project_id  = "my-project-id"
        repo_name = "my-repo-name"
        branch_name = "main"
      }
    }
  }

  service_config {
    # max_instance_count  = 1
    available_memory    = "128M"
    timeout_seconds     = 120
  }

  # Eventarc trigger configuration goes here
  # ... other configuration ...
}
