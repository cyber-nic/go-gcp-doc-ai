# locals {
#   region = "europe-west1"
#   // Firestore is not available in europe-west1, which is were the data exists. The closest is europe-west4.
#   firestore_location_id = "europe-west4"
# }

# // these values must be set in the terraform.tfvars file
# provider "google" {
#   project = "my-project-id"
#   region = local.region
# }

# terraform {
#   required_providers {
#     google = {
#       source  = "hashicorp/google"
#       version = ">= 5.8.0"
#     }
#   }
#   backend "gcs" {
#    bucket  = "my-project-tfstate"
#    prefix  = "terraform/state"
#  }
# }
