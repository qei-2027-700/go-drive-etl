package etl

import (
	"context"
	"testing"

	"github.com/qei-2027-700/go-drive-etl/internal/domain"
	"go.uber.org/mock/gomock"

	bqMock "github.com/qei-2027-700/go-drive-etl/internal/bq/mock"
	driveMock "github.com/qei-2027-700/go-drive-etl/internal/drive/mock"
	repoMock "github.com/qei-2027-700/go-drive-etl/internal/repository/mock"
)

func TestRun_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMock.NewMockFileRepo(ctrl)
	driveClient := driveMock.NewMockDriveClient(ctrl)
	bqClient := bqMock.NewMockBQClient(ctrl)

	ctx := context.Background()

	// 期待される呼び出しを設定
	repo.EXPECT().
		ListPending(gomock.Any()).
		Return([]*domain.File{}, nil)

	// Runを実行
	err := Run(ctx, repo, driveClient, bqClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_CtxCancel(t *testing.T) {

}

