package drive

import (
	"context"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	driveapi "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type Client struct {
	svc *driveapi.Service
}

func NewClient(ctx context.Context) (*Client, error) {
	conf := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Endpoint:     google.Endpoint,
		Scopes:       []string{driveapi.DriveReadonlyScope},
	}

	tok := &oauth2.Token{
		RefreshToken: os.Getenv("GOOGLE_REFRESH_TOKEN"),
	}

	svc, err := driveapi.NewService(ctx,
		option.WithTokenSource(conf.TokenSource(ctx, tok)),
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
