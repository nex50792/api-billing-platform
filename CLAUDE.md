# Claude Instructions

日本語で回答してください。

## ドキュメント記述ルール

README や設定ファイルの数値・バージョン・具体値を記載する際は毎回以下を確認する:

- 内部整合: 複数ファイル間で同じ値になっているか（README ↔ go.mod ↔ Dockerfile ↔ CI 等）
- 外部妥当性: 公式ソース（go.dev、postgresql.org/support/versioning、npm レジストリ等）で現時点の推奨と照合。最新サポート版との乖離、EOL 接近を見落とさない
- 実測: テスト数・エンドポイント数のような具体値はコードから測る。前回の値の踏襲は禁止

## コード・設定ファイルの記述ルール

- 自明なコメント禁止: コマンドや関数名・設定キーから読み取れる内容をコメントで重複させない。書くのは WHY や非自明な制約のみ
- バージョン固定値を README・docs に書かない: ドリフト源になる。コード側（go.mod / package.json / .nvmrc 等）が真実の源

## API 実装ルール

- `docs/openapi.yaml` で定義された request / response は必ず `internal/apispec` の生成型を使う。ハンドラ内で inline struct を定義して JSON 応答するのは禁止（spec と実装の型レベル同期が壊れるため）
- spec を変更したら `make openapi-gen` で `types.gen.go` を再生成する。CI は `go generate` の差分で drift を検出する
