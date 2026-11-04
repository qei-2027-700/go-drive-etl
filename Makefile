.PHONY: mock run-manual-pipeline

# Keep CSV delivery after Gold refresh and verification.
run-manual-pipeline:
	DRIVE_EXPORT_FOLDER_ID= go run ./cmd/worker/
	go run ./cmd/verify_gold --refresh-and-export --data-only

mock:
	mockgen -source=internal/drive/client.go \
					-destination=internal/drive/mock/mock_drive_client.go \
					-package=mock
	mockgen -source=internal/bq/client.go \
					-destination=internal/bq/mock/mock_bq_client.go \
					-package=mock
	mockgen -source=internal/repository/file_repository.go \
					-destination=internal/repository/mock/mock_file_repo.go \
					-package=mock
	mockgen -source=internal/etl/worker_pool.go \
					-destination=internal/etl/mock/mock_worker_pool.go \
					-package=mock
