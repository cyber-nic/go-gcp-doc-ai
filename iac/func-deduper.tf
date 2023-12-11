## iam

resource "google_service_account" "deduper" {
  account_id   = "deduper-sa"
  display_name = "Deduper Service Account"
}

# resource "google_project_iam_member" "firestore_service_agent" {
#   project = var.project_id
#   role    = "roles/firestore.serviceAgent"
#   member  = "serviceAccount:${google_service_account.deduper.email}"
# }


resource "google_project_iam_custom_role" "deduper" {
  role_id     = "deduper"
  title       = "Deduper"
  description = "Custom Role for Deduper Cloud Function Storage Access"
  permissions = [
    "storage.objects.get",
    "storage.objects.list",
  ]
}

resource "google_project_iam_member" "datastore_user" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.deduper.email}"
}


resource "google_storage_bucket_iam_member" "deduper_storage" {
  bucket     = var.deduper_src_bucket_name
  role       = google_project_iam_custom_role.deduper.name
  member     = "serviceAccount:${google_service_account.deduper.email}"
  depends_on = [google_project_iam_custom_role.deduper]
}

# resource "google_project_iam_binding" "dedup_firestore" {
#   project = var.project_id
#   role    = "roles/firestore.serviceAgent"

#   members = [
#      "serviceAccount:${google_service_account.deduper.email}",
#   ]
# }

## deploy

resource "google_storage_bucket" "deduper" {
  name                        = "${var.project_prefix}-func-deploy-deduper"
  location                    = local.region
  uniform_bucket_level_access = true
}

data "archive_file" "deduper" {
  type        = "zip"
  output_path = "/tmp/func-deduper-source.zip"
  source_dir  = "../apps/deduper"
  excludes    = ["local.env", "cmd", "cmd/"]
}

resource "google_storage_bucket_object" "deduper" {
  name   = "dedup-${data.archive_file.deduper.output_sha256}.zip"
  bucket = google_storage_bucket.deduper.name
  source = data.archive_file.deduper.output_path
}


resource "google_cloudfunctions2_function" "deduper" {
  name        = "deduper"
  location    = local.region
  description = "deduper iterates through a bucket and creates a firestore document for each unique file"

  build_config {
    runtime     = "go121"
    entry_point = "Dedup"
    source {
      storage_source {
        bucket = google_storage_bucket.deduper.name
        object = google_storage_bucket_object.deduper.name
      }
    }
  }

  service_config {
    max_instance_count = 1
    min_instance_count = 0
    available_memory   = "256M"
    timeout_seconds    = 540 // max
    environment_variables = {
      DEBUG                           = var.deduper_debug
      GCP_PROJECT_ID                  = var.project_id
      FIRESTORE_DATABASE_ID           = var.project_id
      BUCKET_NAME                     = var.deduper_src_bucket_name
      BUCKET_PREFIX                   = var.deduper_src_bucket_prefix
      BUCKET_CHECKPOINT_NAME          = google_storage_bucket.src_checkpoint.name
      FIRESTORE_IMAGE_COLLECTION_NAME = var.deduper_firestore_image_collection_name
      FIRESTORE_FILE_COLLECTION_NAME  = var.deduper_firestore_files_collection_name
      MAX_FILES                       = var.deduper_max_files
      PROGRESS_COUNT                  = var.deduper_progress_count
    }
    # ingress_settings               = "ALLOW_INTERNAL_ONLY"
    ingress_settings               = "ALLOW_ALL"
    all_traffic_on_latest_revision = true
    service_account_email          = google_service_account.deduper.email
  }
}

output "function_uri" {
  value = google_cloudfunctions2_function.deduper.service_config[0].uri
}