# locals {
#   region        = "europe-west1"
# }

variable "project_id" {
  type = string
}

variable "project_prefix" {
  type = string
}

# nlp
variable "nlp_debug" {
  type    = bool
  default = false
}

# ocr
variable "ocr_debug" {
  type    = bool
  default = false
}

variable "ocr_pubsub_topic_id" {
  type = string
}

variable "ocr_pubsub_subscription_id" {
  type = string
}

variable "ocr_dst_bucket_name" {
  type = string
}

variable "ocr_err_bucket_name" {
  type = string
}

variable "ocr_refs_bucket_name" {
  type = string
}

variable "ocr_doc_ai_processor_location" {
  type = string
}

variable "ocr_doc_ai_processor_id" {
  type = string
}