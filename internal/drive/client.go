package drive

import (
	"context"
	"io"

	driveapi "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type Client struct {
	svc *driveapi.Service
}

type DriveClient interface {
	ListFiles(ctx context.Context, folderID string) ([]*driveapi.File, error)
	DownloadFile(ctx context.Context, fileID string) ([]byte, error)
}

// NewClient は ADC（Application Default Credentials）で Drive API クライアントを作成する。
// サービスアカウントの JSON キーを環境変数 GOOGLE_APPLICATION_CREDENTIALS で指定して使う
// （利用前に対象フォルダをサービスアカウントのメールアドレスに共有しておくこと）。
//
// OAuth 2.0 ユーザー委譲方式に戻したい場合（サービスアカウント方式が使えない環境など）:
//  1. cmd/auth/ を実行してリフレッシュトークンを取得
//  2. .env.example にある GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / GOOGLE_REFRESH_TOKEN の
//     コメントを外して設定する
//  3. 本関数を、golang.org/x/oauth2 の oauth2.Config + TokenSource を使う実装に戻す
//     （このコミット以前の git 履歴を参照。#73 でサービスアカウント方式に切り替えた）
func NewClient(ctx context.Context) (*Client, error) {
	svc, err := driveapi.NewService(ctx,
		option.WithScopes(driveapi.DriveReadonlyScope),
	)
	if err != nil {
		return nil, err
	}

	return &Client{svc: svc}, nil
}

// ListFiles は指定フォルダ内のファイル一覧を返す。folderID が空の場合は全ファイルを対象とする。
func (c *Client) ListFiles(ctx context.Context, folderID string) ([]*driveapi.File, error) {
	q := "trashed = false"
	if folderID != "" {
		q = "'" + folderID + "' in parents and trashed = false"
	}

	var files []*driveapi.File
	pageToken := ""

	for {
		call := c.svc.Files.List().
			Q(q).
			Fields("nextPageToken, files(id, name, mimeType, md5Checksum, modifiedTime)").
			Context(ctx)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		res, err := call.Do()
		if err != nil {
			return nil, err
		}

		files = append(files, res.Files...)

		if res.NextPageToken == "" {
			break
		}
		pageToken = res.NextPageToken
	}

	return files, nil
}

// DownloadFile はバイナリファイル（PDF, CSV など）向け。
// Google Workspace ファイル（Docs/Sheets/Slides）は Export API が必要（#28 参照）。
func (c *Client) DownloadFile(ctx context.Context, fileID string) ([]byte, error) {
	res, err := c.svc.Files.Get(fileID).Context(ctx).Download()
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}
