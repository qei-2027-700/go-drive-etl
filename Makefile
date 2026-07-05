.PHONY: mock

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
