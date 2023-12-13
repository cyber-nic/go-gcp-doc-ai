# ocr pubsub

resource "google_pubsub_topic" "ocr" {
  name = "ocr"
}

resource "google_pubsub_topic" "ocr_dead_letter" {
  name = "ocr-dl"
}

resource "google_pubsub_subscription" "ocr" {
  name  = "ocr-sub"
  topic = google_pubsub_topic.ocr.name

  dead_letter_policy {
    dead_letter_topic     = google_pubsub_topic.ocr_dead_letter.id
    max_delivery_attempts = 10
  }

  ack_deadline_seconds = 10
}

resource "google_pubsub_subscription" "ocr-dl" {
  name  = "ocr-dl-sub"
  topic = google_pubsub_topic.ocr_dead_letter.name
}

## cloud run

resource "google_artifact_registry_repository" "ocr" {
  repository_id = "ocr"
  location      = local.region
  description   = "docker/helm repo for ocr-worker images"
  format        = "docker"

  docker_config {
    immutable_tags = true
  }
}

# service account
resource "google_service_account" "ocr" {
  account_id   = "cloud-run-ocr-sa"
  display_name = "OCR Worker"
  description  = "Cloud Run OCR Worker Service Account"
}

# // https://cloud.google.com/iam/docs/service-agents
resource "google_project_iam_member" "ocr_service_agent" {
  project = var.project_id
  role    = "roles/appengine.serviceAgent"
  member  = "serviceAccount:${google_service_account.ocr.email}"
}

# https://cloud.google.com/pubsub/docs/access-control
resource "google_project_iam_member" "ocr_pubsub_subscriber" {
  project = var.project_id
  role    = "roles/pubsub.subscriber"
  member  = "serviceAccount:${google_service_account.ocr.email}"
}

resource "google_project_iam_member" "ocr_pubsub_viewer" {
  project = var.project_id
  role    = "roles/pubsub.viewer"
  member  = "serviceAccount:${google_service_account.ocr.email}"
}

# https://cloud.google.com/document-ai/docs/access-control/iam-permissions
resource "google_project_iam_member" "ocr_document_ai" {
  project = var.project_id
  role    = "roles/documentai.apiUser"
  member  = "serviceAccount:${google_service_account.ocr.email}"
}

# // https://cloud.google.com/storage/docs/access-control/iam-roles
resource "google_storage_bucket_iam_member" "ocr_err" {
  bucket     = google_storage_bucket.ocr_err.name
  role       = "roles/storage.objectUser"
  member     = "serviceAccount:${google_service_account.ocr.email}"
  depends_on = [google_storage_bucket.ocr_err]
}

resource "google_storage_bucket_iam_member" "ocr_err_attrs" {
  bucket     = google_storage_bucket.ocr_err.name
  member     = "serviceAccount:${google_service_account.nlp.email}"
  role       = google_project_iam_custom_role.bucket_attr_reader.name
  depends_on = [google_project_iam_custom_role.bucket_attr_reader]
}


resource "google_storage_bucket_iam_member" "ocr_refs" {
  bucket     = google_storage_bucket.ocr_refs.name
  role       = "roles/storage.objectUser"
  member     = "serviceAccount:${google_service_account.ocr.email}"
  depends_on = [google_storage_bucket.ocr_refs]
}

resource "google_storage_bucket_iam_member" "ocr_refs_attrs" {
  bucket     = google_storage_bucket.ocr_refs.name
  member     = "serviceAccount:${google_service_account.nlp.email}"
  role       = google_project_iam_custom_role.bucket_attr_reader.name
  depends_on = [google_project_iam_custom_role.bucket_attr_reader]
}


# cloud run
resource "google_cloud_run_service" "ocr" {
  name     = "ocr"
  location = local.region

  template {
    metadata {
      annotations = {
        "autoscaling.knative.dev/minScale" = var.ocr_min_instances
        "autoscaling.knative.dev/maxScale" = var.ocr_max_instances
      }
    }
    spec {
      service_account_name = google_service_account.ocr.email
      containers {
        image = "${local.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.ocr.name}/app:${var.ocr_build_version}"
        ports {
          container_port = 5000
        }

        env {
          name  = "GCP_PROJECT_ID"
          value = var.project_id
        }
        env {
          name  = "PUBSUB_TOPIC_ID"
          value = var.ocr_pubsub_topic_id
        }
        env {
          name  = "PUBSUB_SUBSCRIPTION_ID"
          value = var.ocr_pubsub_subscription_id
        }
        env {
          name  = "DST_BUCKET_NAME"
          value = var.ocr_dst_bucket_name
        }
        env {
          name  = "REFS_BUCKET_NAME"
          value = var.ocr_refs_bucket_name
        }
        env {
          name  = "ERR_BUCKET_NAME"
          value = var.ocr_err_bucket_name
        }
        env {
          name  = "DOC_AI_PROCESSOR_ID"
          value = var.ocr_doc_ai_processor_id
        }
        env {
          name  = "DOC_AI_PROCESSOR_LOCATION"
          value = var.ocr_doc_ai_processor_location
        }

      }

    }
  }

  traffic {
    percent         = 100
    latest_revision = true
  }
}

# outputs
output "ocr_image_name" {
  value = "${google_artifact_registry_repository.ocr.name}/app:latest"
}