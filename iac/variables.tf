# locals {
#   region        = "europe-west1"
# }

variable "project_id" {
  type = string
}

variable "project_prefix" {
  type = string
}

# deduper
variable "deduper_debug" {
  type = bool
  default = false
}
variable "deduper_src_bucket_name" {
  type = string
}
variable "deduper_src_bucket_prefix" {
  type = string
}
variable "deduper_firestore_image_collection_name" {
  type = string
}
variable "deduper_firestore_files_collection_name" {
  type = string
}
variable "deduper_max_files" {
  type = string
}