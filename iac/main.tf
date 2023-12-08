
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

// ocr pubsub

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
    dead_letter_topic = google_pubsub_topic.example_dead_letter.id
    max_delivery_attempts = 10
  }

  # ack_deadline_seconds = 10

  # labels = {
  #   foo = "bar"
  # }

  push_config {
    push_endpoint = "https://example.com/push"

    attributes = {
      x-goog-version = "v1"
    }
  }
}