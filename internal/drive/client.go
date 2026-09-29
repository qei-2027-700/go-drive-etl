package drive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	driveapi "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

type Client struct {
	svc *driveapi.Service
}

const folderMimeType = "application/vnd.google-apps.folder"

type DriveClient interface {
	ListFiles(ctx context.Context, folderID string) ([]*driveapi.File, error)
	DownloadFile(ctx context.Context, fileID string) ([]byte, error)
	DownloadGoogleWorkspaceFile(ctx context.Context, fileID string, exportMimeType string) ([]byte, error)
	UpsertFile(ctx context.Context, folderID, name, mimeType string, content []byte) error
}

// NewClient creates an OAuth user-delegated client when all OAuth environment
// variables are configured. Otherwise it uses Application Default Credentials,
// which is appropriate for a service account and shared drives.
func NewClient(ctx context.Context) (*Client, error) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	refreshToken := os.Getenv("GOOGLE_REFRESH_TOKEN")

	var opts []option.ClientOption
	if clientID != "" && clientSecret != "" && refreshToken != "" {
		conf := &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     google.Endpoint,
			Scopes:       []string{driveapi.DriveScope},
		}
		opts = append(opts, option.WithTokenSource(conf.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})))
	} else {
		opts = append(opts, option.WithScopes(driveapi.DriveScope))
	}

	svc, err := driveapi.NewService(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{svc: svc}, nil
}

// UpsertFile updates a same-named file in folderID, or creates it when absent.
// Stable names make report delivery idempotent across worker runs.
func (c *Client) UpsertFile(ctx context.Context, folderID, name, mimeType string, content []byte) error {
	if folderID == "" {
		return fmt.Errorf("export folder ID is required")
	}
	q := fmt.Sprintf("'%s' in parents and name = '%s' and trashed = false", driveQueryLiteral(folderID), driveQueryLiteral(name))
	files, err := c.svc.Files.List().Q(q).Fields("files(id, name)").Context(ctx).Do()
	if err != nil {
		return err
	}
	media := googleapi.ContentType(mimeType)
	if len(files.Files) > 0 {
		_, err = c.svc.Files.Update(files.Files[0].Id, &driveapi.File{Name: name, MimeType: mimeType}).Media(bytes.NewReader(content), media).Context(ctx).Do()
		return err
	}
	_, err = c.svc.Files.Create(&driveapi.File{Name: name, MimeType: mimeType, Parents: []string{folderID}}).Media(bytes.NewReader(content), media).Context(ctx).Do()
	return err
}

func driveQueryLiteral(value string) string {
	return strings.ReplaceAll(value, "'", "\\'")
}

// ListFiles は指定フォルダ内のファイル一覧を返す。folderID が空の場合は全ファイルを対象とする。
func (c *Client) ListFiles(ctx context.Context, folderID string) ([]*driveapi.File, error) {
	q := "trashed = false and mimeType != '" + folderMimeType + "'"
	if folderID != "" {
		q = "'" + folderID + "' in parents and trashed = false and mimeType != '" + folderMimeType + "'"
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

		// The query excludes folders, but retain this guard so an unexpected API
		// response cannot enqueue a non-downloadable folder for processing.
		for _, file := range res.Files {
			if file.MimeType != folderMimeType {
				files = append(files, file)
			}
		}

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

func (c *Client) DownloadGoogleWorkspaceFile(ctx context.Context, fileID string, exportMimeType string) ([]byte, error) {
	res, err := c.svc.Files.Export(fileID, exportMimeType).Context(ctx).Download()
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
