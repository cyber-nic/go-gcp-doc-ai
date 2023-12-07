provider "google" {
  project = "your-gcp-project-id"
  region  = "eu-west4"
}

terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.8.0"
    }
  }
}
