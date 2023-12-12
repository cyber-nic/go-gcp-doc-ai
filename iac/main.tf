
resource "google_firestore_database" "database" {
  project                 = var.project_id
  name                    = var.project_id
  location_id             = local.firestore_location_id
  type                    = "FIRESTORE_NATIVE"
  delete_protection_state = "DELETE_PROTECTION_ENABLED"
}

# test buckets

resource "google_storage_bucket" "src_checkpoint_test" {
  name          = "${var.project_prefix}-src-checkpoint-test"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "ocr_refs_test" {
  name          = "${var.project_prefix}-ocr-refs-test"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "ocr_err_test" {
  name          = "${var.project_prefix}-ocr-err-test"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "ocr_data_test" {
  name          = "${var.project_prefix}-ocr-data-test"
  location      = local.region
  force_destroy = true
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

resource "google_storage_bucket" "dispatcher_checkpoint_test" {
  name          = "${var.project_prefix}-dispatcher-checkpoint-test"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "dispatcher_refs" {
  name          = "${var.project_prefix}-dispatcher-refs"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "dispatcher_refs_test" {
  name          = "${var.project_prefix}-dispatcher-refs-test"
  location      = local.region
  force_destroy = true
}

// usedby ocr
resource "google_storage_bucket" "ocr_err" {
  name          = "${var.project_prefix}-ocr-err"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "ocr_data" {
  name          = "${var.project_prefix}-ocr-data"
  location      = local.region
  force_destroy = true
}

resource "google_storage_bucket" "nlp_err" {
  name          = "${var.project_prefix}-nlp-err"
  location      = local.region
  force_destroy = true
}

// used by nlp
resource "google_storage_bucket" "nlp_data" {
  name          = "${var.project_prefix}-nlp-data"
  location      = local.region
  force_destroy = true
}

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