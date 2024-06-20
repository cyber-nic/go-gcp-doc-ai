// iam
resource "google_project_service" "iam" {
  project = var.project_id
  service = "iam.googleapis.com"
}

// cloudbuild
resource "google_project_service" "cloudbuild" {
  project = var.project_id
  service = "cloudbuild.googleapis.com"
}

// eventarc
resource "google_project_service" "eventarc" {
  project = var.project_id
  service = "eventarc.googleapis.com"
}

// firestore
resource "google_project_service" "firestore" {
  project = var.project_id
  service = "firestore.googleapis.com"
}

// cloudfunctions
resource "google_project_service" "cloudfunctions" {
  project = var.project_id
  service = "cloudfunctions.googleapis.com"
}

// storage-api
resource "google_project_service" "storage_api" {
  project = var.project_id
  service = "storage-api.googleapis.com"
}

// run
resource "google_project_service" "run" {
  project = var.project_id
  service = "run.googleapis.com"
}

// pubsub
resource "google_project_service" "pubsub" {
  project = var.project_id
  service = "pubsub.googleapis.com"
}

// vision
resource "google_project_service" "vision" {
  project = var.project_id
  service = "vision.googleapis.com"
}

// documentai
resource "google_project_service" "documentai" {
  project = var.project_id
  service = "documentai.googleapis.com"
}

// language
resource "google_project_service" "language" {
  project = var.project_id
  service = "language.googleapis.com"
}
