
# buckets
resource "google_storage_bucket" "src" {
  name          = "source"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "src_refs" {
  name          = "src-refs"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "ocr_err" {
  name          = "ocr-err"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "ocr_output" {
  name          = "ocr-output"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "nlp_err" {
  name          = "nlp-err"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "nlp_output" {
  name          = "nlp-output"
  location      = local.region
  force_destroy = true
}

resource "google_pubsub_topic" "ocr_dispatch" {
  name = "ocr"
}



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
