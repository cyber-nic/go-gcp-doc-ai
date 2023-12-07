
resource "google_service_account" "ocr" {
  account_id   = "ocr-worker-sa"
  display_name = "Test Service Account"
}

data "archive_file" "ocr" {
  type        = "zip"
  output_path = "/tmp/func-ocr-source.zip"
  source_dir  = "../ocr-worker/"
}

resource "google_storage_bucket_object" "ocr" {
  name   = "function-source.zip"
  bucket = google_storage_bucket.default.name
  source = data.archive_file.default.output_path
}

resource "google_storage_bucket" "ocr" {
  name                        = "ocr-func-source"
  location                    = local.region
  uniform_bucket_level_access = true
}


resource "google_cloudfunctions2_function" "ocr" {
  name        = "ocr-worker"
  location    = local.region
  description = "ocr worker submits batches of images for ocr processing"

  build_config {
    runtime     = "go121"
    entry_point = "handler"
    environment_variables = {
      SOURCE_BUCKET = "build_test"
    }
    source {
      storage_source {
        bucket = google_storage_bucket.ocr.name
        object = google_storage_bucket_object.ocr.name
      }
    }
  }

  service_config {
    max_instance_count = 5
    min_instance_count = 0
    available_memory   = "128M"
    timeout_seconds    = 120
    environment_variables = {
      SERVICE_CONFIG_TEST = "config_test"
    }
    ingress_settings               = "ALLOW_INTERNAL_ONLY"
    all_traffic_on_latest_revision = true
    service_account_email          = google_service_account.ocr.email
  }

  event_trigger {
    trigger_region = local.region
    event_type     = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic   = google_pubsub_topic.ocr_dispatch.id
    retry_policy   = "RETRY_POLICY_RETRY"
  }
}