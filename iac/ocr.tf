
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