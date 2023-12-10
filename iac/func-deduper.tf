resource "google_service_account" "deduper" {
  account_id   = "deduper-sa"
  display_name = "Deduper Service Account"
}

resource "google_storage_bucket" "deduper" {
  name                        = "deduper-func-source"
  location                    = local.region
  uniform_bucket_level_access = true
}

data "archive_file" "deduper" {
  type        = "zip"
  output_path = "/tmp/func-deduper-source.zip"
  source_dir  = "../apps/deduper/"
}

resource "google_storage_bucket_object" "deduper" {
  name   = "function-source.zip"
  bucket = google_storage_bucket.deduper.name
  source = data.archive_file.deduper.output_path
}

resource "google_cloudfunctions2_function" "deduper" {
  name        = "deduper"
  location    = local.region
  description = "deduper iterates through a bucket and creates a firestore document for each unique file"

  build_config {
    runtime     = "go121"
    entry_point = "handler"
    environment_variables = {
      SOURCE_BUCKET                   = "build_test"
      PUBSUB_TOPIC                    = "ocr-stream"
      DEBUG                           = "false"
      GCP_PROJECT_ID                  = var.project_id
      BUCKET_NAME                     = google_storage_bucket.deduper.name
      BUCKET_PREFIX                   = var.deduper_src_bucket_prefix
      FIRESTORE_IMAGE_COLLECTION_NAME = var.deduper_firestore_image_collection_name
      FIRESTORE_FILE_COLLECTION_NAME  = var.deduper_firestore_files_collection_name
      MAX_FILES                       = var.deduper_max_files
    }
    source {
      storage_source {
        bucket = google_storage_bucket.deduper.name
        object = google_storage_bucket_object.deduper.name
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
    service_account_email          = google_service_account.deduper.email
  }


}