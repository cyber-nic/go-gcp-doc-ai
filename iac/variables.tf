# locals {
#   region        = "europe-west1"
# }

variable "project_id" {
  type = string
}

variable "resource_name_prefix" {
  type = string
}

# ocr
variable "ocr_min_instances" {
  type    = number
  default = 0
}

variable "ocr_max_instances" {
  type    = number
  default = 5
}

variable "ocr_doc_ai_min_req_seconds" {
  type    = number
  default = 60
}

variable "ocr_debug" {
  type    = bool
  default = false
}

variable "ocr_build_version" {
  type = string
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

# nlp
variable "nlp_debug" {
  type    = bool
  default = false
}