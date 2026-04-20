# API Billing Platform

Go + PostgreSQL + React Router による SaaS 型 API 課金プラットフォーム

登場する 3 種類のユーザー:
- Provider: API を提供する側
- Customer: API を利用する側
- Admin: プラットフォーム運営者

## スタック

- バックエンド: Go / chi / PostgreSQL
- フロントエンド: React Router / React / TypeScript / Tailwind CSS
- 決済: Stripe（Checkout / Billing Portal / Webhook）
- メール: SMTP
- インフラ: Docker Compose、GitHub Actions CI

## 主な実装

### Customer 機能
- サインアップ（メール認証コード方式）
- ログイン／ログアウト（セッション Cookie）
- パスワードリセット（メール経由）
- パスワード変更（他セッション無効化）
- API キー発行・削除・一覧
- Stripe Checkout でのプラン変更
- Billing Portal でのサブスクリプション管理

### Provider 機能
- 管理者による招待 → パスワード設定リンク配信で有効化
- ログイン／パスワード変更
- 提供 API の CRUD
- API キーの検証エンドポイント（自身の提供 API に限定）
- 利用状況の可視化

### Admin 機能
- ログイン／パスワード変更
- Customer 一覧
- Provider 追加・削除
- メール送信ログ閲覧

### プラットフォーム機能
- API キー認証
- プランごとの月間リクエスト上限（Free 100 / Pro 10,000）
- Stripe Webhook 連携（チェックアウト完了・サブスクリプション更新・支払失敗に対応）

## 開発セットアップ

### 前提条件

- Docker & Docker Compose
- Node.js（バージョンは `frontend/.nvmrc` 参照）
- Go（バージョンは `backend/go.mod` 参照。ローカル開発のみ。Docker 内でもOK）

### クイックスタート

```bash
# 1. 環境変数ファイルを作成
cp backend/.env.example backend/.env
cp frontend/.env.example frontend/.env.local

# 2. DB + API サービス起動
make up

# 3. DB スキーマ適用
make migrate

# 4. 起動ログ確認
make logs

# 5. フロントエンド起動（別ターミナル）
cd frontend
npm ci
npm run dev
# → http://localhost:5173
```

### 停止

```bash
make down
```

## テスト・Lint

```bash
# 全体
make test
make lint

# 個別
make test-backend
make test-frontend
make lint-backend
make lint-frontend

# OpenAPI と実装の乖離検証
make test-openapi
```

その他の target は `Makefile` 参照。

## API 仕様

- 仕様ファイル: [docs/openapi.yaml](docs/openapi.yaml)
- 対話的 UI: `make openapi` → http://localhost:8000
- Go 型再生成: `make openapi-gen`（仕様を編集したら実行、CI で差分検出）

### CI での自動検証

- OpenAPI の構文: Redocly CLI で lint
- 生成型の同期: `go generate` 結果と commit 済み `types.gen.go` の差分を検出
- 実装との整合: [schemathesis](https://schemathesis.io) がプロパティベースのテストを仕様から自動生成し、実サーバーに対して実行（レスポンス形状・ステータスコード・スキーマ違反を検出）

## アーキテクチャ概要

```
 Browser (React Router)
        ↓
 [SecurityHeaders + CSRF + CORS + RateLimit]
        ↓
 Handler Layer  ───→  Mailer (SMTP)
        ↓          ───→  Stripe SDK
 Service Layer
        ↓
 Repository Layer
        ↓
 PostgreSQL
```

## 設計方針

1. 3 ロール分離: Customer / Provider / Admin の認証・セッションを独立管理
2. レイヤードアーキテクチャ: Handler → Service → Repository（インターフェース駆動で疎結合）
3. セキュリティ: API キーは SHA-256 ハッシュ保存、パスワードは bcrypt、Stripe Webhook は署名検証
4. 冪等性: Stripe Webhook の重複処理防止、パスワード変更時の全セッション無効化
5. レースコンディション対策: API キー使用量の上限チェック＋インクリメントを単一 SQL（CTE）で原子的に実行

## 技術選定

| 領域 | 採用 | 理由 |
|---|---|---|
| Web フレームワーク | chi | 標準 `net/http` 互換、ミドルウェア合成が柔軟 |
| OpenAPI 仕様駆動 | spec-first（手書き yaml） | 実装と独立した真実の源、SDK 生成・契約テスト前提 |
| OpenAPI 閲覧 | Swagger UI | OpenAPI 生態系の de facto、Try it out で対話的に叩ける |
| 契約テスト | schemathesis | Property-based に OpenAPI からテスト自動生成、Go プロジェクトでの事実上唯一解 |
| フロント FW | React Router（Remix 後継） | SSR + `loader` による初期データ取得、型付きルーティング |
| Lint | golangci-lint / ESLint（flat config） | 両言語の業界標準 |
| CI | GitHub Actions | 無料・無メンテ |

## 今後の拡張候補

- [ ] E2E テスト（Playwright）
- [ ] OpenAPI 仕様の公開
- [ ] Terraform で AWS インフラ定義
- [ ] 構造化ログ（`log/slog` 移行）
- [ ] 監査ログ機能
