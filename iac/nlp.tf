## iam

resource "google_service_account" "nlp" {
  account_id   = "nlp-sa"
  display_name = "nlp Service Account"
}

// ocr_data is the nlp input
resource "google_storage_bucket_iam_member" "nlp_data_viewer" {
  bucket = google_storage_bucket.ocr_data.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.nlp.email}"
}

// nlp_data is the nlp output
resource "google_storage_bucket_iam_member" "nlp_data_writer" {
  bucket = google_storage_bucket.nlp_data.name
  role   = "roles/storage.objectUser"
  member = "serviceAccount:${google_service_account.nlp.email}"
}

// nlp_err is the nlp error output
resource "google_storage_bucket_iam_member" "nlp_err_writer" {
  bucket = google_storage_bucket.nlp_err.name
  role   = "roles/storage.objectUser"
  member = "serviceAccount:${google_service_account.nlp.email}"
}

// allow eventarc to invoke function
resource "google_project_iam_member" "nlp_invoke_func" {
  project = var.project_id
  role    = "roles/run.invoker"
  member  = "serviceAccount:${google_service_account.nlp.email}"
}

resource "google_project_iam_member" "nlp_datastore_user" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.nlp.email}"
}

resource "google_project_iam_member" "nlp_eventarc_receiver" {
  project = var.project_id
  role    = "roles/eventarc.eventReceiver"
  member  = "serviceAccount:${google_service_account.nlp.email}"
}


## deploy

resource "google_storage_bucket" "nlp_deploy" {
  name                        = "${var.resource_name_prefix}-func-deploy-nlp"
  location                    = local.region
  uniform_bucket_level_access = true
}

data "archive_file" "nlp" {
  type        = "zip"
  output_path = "/tmp/func-nlp-source.zip"
  source_dir  = "../apps/nlp-worker"
  excludes    = ["local.env", "cmd", "cmd/"]
}

resource "google_storage_bucket_object" "nlp_deploy" {
  name   = "nlp-${data.archive_file.nlp.output_sha256}.zip"
  bucket = google_storage_bucket.nlp_deploy.name
  source = data.archive_file.nlp.output_path
}

resource "google_cloudfunctions2_function" "nlp" {
  name        = "nlp"
  location    = local.region
  description = "nlp performs NLP on the text extracted from the images"
  labels = {
    app = "nlp"
  }

  build_config {
    runtime     = "go121"
    entry_point = "Handler"
    docker_repository = "projects/${var.project_id}/locations/${local.region}/repositories/gcf-artifacts"
    source {
      storage_source {
        bucket = google_storage_bucket.nlp_deploy.name
        object = google_storage_bucket_object.nlp_deploy.name
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
    service_account_email          = google_service_account.nlp.email

    environment_variables = {
      DEBUG           = var.nlp_debug
      GCP_PROJECT_ID  = var.project_id
      ERR_BUCKET_NAME = google_storage_bucket.nlp_err.name
      DST_BUCKET_NAME = google_storage_bucket.nlp_data.name
      LOG_EXECUTION_ID = true
    }
  }

  event_trigger {
    trigger_region        = local.region
    event_type            = "google.cloud.storage.object.v1.finalized"
    retry_policy          = "RETRY_POLICY_DO_NOT_RETRY" #"RETRY_POLICY_RETRY"
    service_account_email = google_service_account.nlp.email

    event_filters {
      attribute = "bucket"
      value     = google_storage_bucket.ocr_data.name
    }
  }
}

output "nlp_func_uri" {
  value = google_cloudfunctions2_function.nlp.service_config[0].uri
}