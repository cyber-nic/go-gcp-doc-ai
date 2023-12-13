# Makefile
SHELL := /bin/bash

build:
	# requires source ./local.env
	gcloud auth configure-docker "${GCR_REGION}"
	docker build -t "${OCR_ARTIFACT_REGISTRY_URI}/app:${OCR_BUILD_VERSION}" -f ./apps/ocr-worker/Dockerfile .
	docker images | head -n2
	docker push "${OCR_ARTIFACT_REGISTRY_URI}/app:${OCR_BUILD_VERSION}"
	head -n -1 ./iac/terraform.tfvars > ./tmp.tfvars ; mv tmp.tfvars ./iac/terraform.tfvars
	echo "ocr_build_version = \"${OCR_BUILD_VERSION}\"" >> ./iac/terraform.tfvars

deploy:
	# requires source ./local.env
	# requires source ./iac/terraform.tfvars
	cd ./iac && terraform apply -auto-approve

plan:
	# requires source ./local.env
	# requires source ./iac/terraform.tfvars
	cd ./iac && terraform apply -auto-approve