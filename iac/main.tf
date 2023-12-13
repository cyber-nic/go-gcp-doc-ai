
resource "google_firestore_database" "database" {
  project                 = var.project_id
  name                    = var.project_id
  location_id             = local.firestore_location_id
  type                    = "FIRESTORE_NATIVE"
  delete_protection_state = "DELETE_PROTECTION_ENABLED"
}

# buckets

// used by deduper
resource "google_storage_bucket" "src_checkpoint" {
  name          = "${var.project_prefix}-src-checkpoint"
  location      = local.region
  force_destroy = true
}

// used by dispatcher
resource "google_storage_bucket" "dispatcher_checkpoint" {
  name          = "${var.project_prefix}-dispatcher-checkpoint"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "dispatcher_refs" {
  name          = "${var.project_prefix}-dispatcher-refs"
  location      = local.region
  force_destroy = true
}

// used by ocr-worker
resource "google_storage_bucket" "ocr_err" {
  name          = "${var.project_prefix}-ocr-err"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "ocr_refs" {
  name          = "${var.project_prefix}-ocr-refs"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "ocr_data" {
  name          = "${var.project_prefix}-ocr-data"
  location      = local.region
  force_destroy = true
}

// used by nlp-worker
resource "google_storage_bucket" "nlp_data" {
  name          = "${var.project_prefix}-nlp-data"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "nlp_err" {
  name          = "${var.project_prefix}-nlp-err"
  location      = local.region
  force_destroy = true
}
