# Makefile

build:
	# requires source ./local.env
	gcloud auth configure-docker $REGION
	docker build -t "${OCR_ARTIFACT_REGISTRY_URI}/app:${OCR_BUILD_VERSION}" -f ./apps/ocr-worker/Dockerfile .
	docker images | head -n2
	docker push "${OCR_ARTIFACT_REGISTRY_URI}/app:${OCR_BUILD_VERSION}"
