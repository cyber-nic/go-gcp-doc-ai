

# https://cloud.google.com/storage/docs/reporting-changes#terraform

# resource "google_project_iam_member" "ocr_bucket_events" {
#   project = var.project_id
#   role    = "roles/pubsub.publisher"
#   member  = "serviceAccount:${data.google_storage_project_service_account.gcs_account.email_address}"
# }

## iam

resource "google_service_account" "ocr" {
  account_id   = "ocr-sa"
  display_name = "ocr Service Account"
}

// source is the ocr input
resource "google_storage_bucket_iam_member" "ocr_data_viewer" {
  bucket = data.google_storage_bucket.sanitized.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.ocr.email}"
}

// ocr_data is the ocr output
resource "google_storage_bucket_iam_member" "ocr_data_writer" {
  bucket = google_storage_bucket.ocr_data.name
  role   = "roles/storage.objectUser"
  member = "serviceAccount:${google_service_account.ocr.email}"
}

// ocr_data is the ocr error output
resource "google_storage_bucket_iam_member" "ocr_err_writer" {
  bucket = google_storage_bucket.ocr_err.name
  role   = "roles/storage.objectUser"
  member = "serviceAccount:${google_service_account.ocr.email}"
}

// allow eventarc to invoke function
resource "google_project_iam_member" "ocr_invoke_func" {
  project = var.project_id
  role    = "roles/run.invoker"
  member  = "serviceAccount:${google_service_account.ocr.email}"
}

resource "google_project_iam_member" "ocr_datastore_user" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.ocr.email}"
}

resource "google_project_iam_member" "ocr_eventarc_receiver" {
  project = var.project_id
  role    = "roles/eventarc.eventReceiver"
  member  = "serviceAccount:${google_service_account.ocr.email}"
}


## deploy

resource "google_storage_bucket" "ocr_deploy" {
  name                        = "${var.resource_name_prefix}-func-deploy-ocr"
  location                    = local.region
  uniform_bucket_level_access = true
}

data "archive_file" "ocr" {
  type        = "zip"
  output_path = "/tmp/func-ocr-source.zip"
  source_dir  = "../apps/ocr-worker"
  excludes    = ["bin", "cmd", "cmd/"]
}

resource "google_storage_bucket_object" "ocr_deploy" {
  name   = "ocr-${data.archive_file.ocr.output_sha256}.zip"
  bucket = google_storage_bucket.ocr_deploy.name
  source = data.archive_file.ocr.output_path
}

resource "google_cloudfunctions2_function" "ocr" {
  name        = "ocr"
  location    = local.region
  description = "ocr performs an async ocr operation on each file uploaded to the source bucket"
  labels = {
    app = "ocr"
  }

  build_config {
    runtime     = "go121"
    entry_point = "Handler"
    docker_repository = "projects/${var.project_id}/locations/${local.region}/repositories/gcf-artifacts"
    source {
      storage_source {
        bucket = google_storage_bucket.ocr_deploy.name
        object = google_storage_bucket_object.ocr_deploy.name
      }
    }
  }

  service_config {
    available_memory   = "256M"
    timeout_seconds    = 120
    min_instance_count = 0
    max_instance_count = 500

    all_traffic_on_latest_revision = true
    ingress_settings               = "ALLOW_INTERNAL_ONLY"
    service_account_email          = google_service_account.ocr.email

    environment_variables = {
      DEBUG           = var.ocr_debug
      GCP_PROJECT_ID  = var.project_id
      ERR_BUCKET_NAME = google_storage_bucket.ocr_err.name
      DST_BUCKET_NAME = google_storage_bucket.ocr_data.name
      LOG_EXECUTION_ID = true
    }
  }

  event_trigger {
    trigger_region        = local.region
    event_type            = "google.cloud.storage.object.v1.finalized"
    retry_policy          = "RETRY_POLICY_DO_NOT_RETRY" #"RETRY_POLICY_RETRY" 
    service_account_email = google_service_account.ocr.email

    event_filters {
      attribute = "bucket"
      value     = data.google_storage_bucket.sanitized.name
    }
  }
}

output "ocr_func_uri" {
  value = google_cloudfunctions2_function.ocr.service_config[0].uri
}
