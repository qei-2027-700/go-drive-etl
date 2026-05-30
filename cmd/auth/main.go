package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	driveapi "google.golang.org/api/drive/v3"
)

func main() {
	_ = godotenv.Load()

	// 1. 環境変数から Client ID と Secret を取得
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatal("GOOGLE_CLIENT_ID と GOOGLE_CLIENT_SECRET を環境変数に設定してください。")
	}

	// 2. OAuth2 設定を作成
	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		// リダイレクト先としてローカルサーバーを指定
		RedirectURL: "http://localhost:8080/callback",
		// Drive API の読み取り権限を要求
		Scopes: []string{driveapi.DriveReadonlyScope},
	}

	// 3. リフレッシュトークンを取得するため、AccessTypeOffline と ApprovalForce を設定
	authURL := conf.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	fmt.Println("以下のURLをブラウザで開き、認証を完了させてください:")
	fmt.Printf("\n%s\n\n", authURL)

	// 4. コールバックを受け取るための一時的なローカルサーバーを起動
	codeChan := make(chan string)
	srv := &http.Server{Addr: ":8080"}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			fmt.Fprintf(w, "認証が成功しました！ターミナルに戻ってリフレッシュトークンを確認してください。このタブは閉じて構いません。")
			codeChan <- code
		} else {
			fmt.Fprintf(w, "コードの取得に失敗しました。")
		}
	})

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("ローカルサーバーの起動に失敗: %v", err)
		}
	}()

	// ユーザーが認証を終えて code が送られてくるのを待つ
	code := <-codeChan

	// サーバーをシャットダウン
	srv.Shutdown(context.Background())

	// 5. 認可コードをトークン（リフレッシュトークン含む）に交換
	tok, err := conf.Exchange(context.Background(), code)
	if err != nil {
		log.Fatalf("トークンの交換に失敗しました: %v", err)
	}

	// 6. 取得したリフレッシュトークンを画面に表示
	fmt.Println("==================================================")
	fmt.Println("認証成功！以下のリフレッシュトークンを .env に設定してください。")
	fmt.Printf("GOOGLE_REFRESH_TOKEN=%s\n", tok.RefreshToken)
	fmt.Println("==================================================")
}
