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