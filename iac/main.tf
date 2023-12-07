
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
resource "google_pubsub_schema" "ocr" {
  name       = "events-schema"
  type       = "AVRO"
  definition = file("../apps/dispatcher/event-schema.json")
}

resource "google_pubsub_topic" "ocr_dispatch" {
  name = "ocr"
}
