
# resource "google_service_account" "default" {
#   account_id   = "gpu-sa"
#   display_name = "Custom SA for VM Instance"
# }

# resource "google_compute_instance" "with_gpu" {
#   name         = "gpu"
#   machine_type = "n1-standard-1"
#   zone         = "${local.region}-b"

#   boot_disk {
#     initialize_params {
#       # image = "debian-cloud/debian-12"
#       # gcloud compute images list
#       image = "debian-cloud/debian-12"
#       size  = 100
#     }
#   }

#   network_interface {
#     network = "default"
#     access_config {
#       network_tier = "STANDARD"
#       // Ephemeral IP
#     }
#   }

#   // https://cloud.google.com/compute/docs/gpus
#   guest_accelerator {
#     type  = "nvidia-tesla-t4"
#     count = 1
#   }

#   scheduling {
#     on_host_maintenance = "TERMINATE"
#     automatic_restart   = true
#   }

#   service_account {
#     # Google recommends custom service accounts that have cloud-platform scope and permissions granted via IAM Roles.
#     email  = google_service_account.default.email
#     scopes = ["cloud-platform"]
#   }

#   #  metadata = {
#   #   startup-script-url = "gs://<bucket>/path/to/file"
#   # }
#   #   metadata = {
#   #     startup-script = <<-EOF
#   #   sudo apt install linux-headers-$(uname -r) build-essential python3-distutils tmux git curl vim

#   #   # install drivers
#   #   # https://cloud.google.com/compute/docs/gpus/install-grid-drivers
#   #   curl -O https://storage.googleapis.com/nvidia-drivers-us-public/GRID/vGPU16.2/NVIDIA-Linux-x86_64-535.129.03-grid.run
#   #   sudo bash NVIDIA-Linux-x86_64-535.129.03-grid.run

#   #   sudo apt install libpng-dev libjpeg-dev libtiff-dev imagemagick
#   #   sudo apt install sox ffmpeg libcairo2 libcairo2-dev
#   #   sudo apt install libcairo2-dev pkg-config python3-dev
#   #   sudo apt install libgirepository1.0-dev

#   #   sudo curl "https://bootstrap.pypa.io/get-pip.py" -o "get-pip.py"
#   #   sudo python get-pip.py
#   #   pip install -r requirements.txt


#   # SSH
#   # https://cloud.google.com/compute/docs/connect/root-ssh#gcloud
#   #   EOF
#   #   }

#   # metadata = {
#   #   ssh-keys = "${var.gce_ssh_user}:${file(var.gce_ssh_pub_key_file)}"
#   # }
# }