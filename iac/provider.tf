locals {
  region = "europe-west1"
  state_bucket_name = "foo-bar"
}

// these values must be set in the terraform.tfvars file
provider "google" {
  project = "your-gcp-project-id"
  # region  = "europe-west1"
  region = local.region
}

terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.8.0"
    }
  }
  backend "gcs" {
   bucket  = "BUCKET_NAME"
   prefix  = "terraform/state"
 }
}

resource "google_storage_bucket" "tfstate" {
  name          = local.state_bucket_name
  force_destroy = false
  location      = local.region
  storage_class = "STANDARD"
  versioning {
    enabled = true
  }
  encryption {
    default_kms_key_name = google_kms_crypto_key.terraform_state_bucket.id
  }
  depends_on = [
    google_project_iam_member.default
  ]
}