resource "google_service_account" "dispatcher" {
  account_id   = "test-gcf-sa"
  display_name = "Test Service Account"
}

resource "google_storage_bucket" "dispatcher" {
  name                        = "dispatcher-func-source"
  location                    = local.region
  uniform_bucket_level_access = true
}

data "archive_file" "dispatcher" {
  type        = "zip"
  output_path = "/tmp/func-nlp-source.zip"
  source_dir  = "../apps/nlp-worker/"
}

resource "google_storage_bucket_object" "dispatcher" {
  name   = "function-source.zip"
  bucket = google_storage_bucket.dispatcher.name
  source = data.archive_file.dispatcher.output_path
}

resource "google_cloudfunctions2_function" "dispatcher" {
  name        = "dispatcher"
  location    = local.region
  description = "dispatcher recives manual trigger and creates pubsub event for batches of images"

  build_config {
    runtime     = "go121"
    entry_point = "handler"
    environment_variables = {
      SOURCE_BUCKET = "build_test"
      PUBSUB_TOPIC = "ocr-stream"
    }
    source {
      storage_source {
        bucket = google_storage_bucket.dispatcher.name
        object = google_storage_bucket_object.dispatcher.name
      }
    }
    # source {
    #   repo_source {
    #     project_id  = "my-project-id"
    #     repo_name = "my-repo-name"
    #     branch_name = "main"
    #   }
    # }
  }

  service_config {
    max_instance_count = 1
    min_instance_count = 0
    available_memory   = "256M"
    timeout_seconds    = 540 // max
    environment_variables = {
      SERVICE_CONFIG_TEST = "config_test"
    }
    ingress_settings               = "ALLOW_INTERNAL_ONLY"
    all_traffic_on_latest_revision = true
    service_account_email          = google_service_account.dispatcher.email
  }

  
}