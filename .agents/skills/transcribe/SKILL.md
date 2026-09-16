---
name: transcribe
description: ローカル音声・動画ファイルをopenai-whisperで文字起こしする。ユーザーが「文字起こしして」「音声を読んで」と依頼したときに使う。
---

# Transcribe

指定されたローカル音声または動画ファイルを `openai-whisper` で文字起こしする。

1. 対象ファイルが存在すること、`ffmpeg` と `whisper` パッケージが使えることを確認する。
2. 未導入のパッケージやモデルのダウンロードが必要なら、容量とネットワーク利用を説明してユーザーの承認を得る。
3. 日本語は `language="ja"` を指定する。通常は `base` モデルを使い、長時間音声や精度重視の場合に `small` への変更を提案する。

```bash
python3 - <<'PY'
import whisper

model = whisper.load_model("base")
result = model.transcribe("<audio-path>", language="ja")
print(result["text"])
PY
```

文字起こし結果を表示し、必要に応じて要約や話者・時刻情報の整形を行う。
