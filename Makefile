.PHONY: up down logs test test-backend test-frontend test-openapi lint lint-backend lint-frontend migrate migrate-down openapi openapi-gen clean

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f api

test: test-backend test-frontend

test-backend:
	docker compose run --rm builder

test-frontend:
	cd frontend && npm run typecheck && npm run build

test-openapi:
	docker run --rm --network host -v $(CURDIR)/docs:/docs schemathesis/schemathesis:stable \
		run /docs/openapi.yaml --url http://localhost:8080 \
		--checks response_schema_conformance --phases fuzzing --max-examples 10

lint: lint-backend lint-frontend

lint-backend:
	cd backend && golangci-lint run ./...

lint-frontend:
	cd frontend && npm run lint

migrate:
	docker compose run --rm migrate up

migrate-down:
	docker compose run --rm migrate down 1

openapi:
	cd docs && python3 -m http.server 8000

openapi-gen:
	cd backend/internal/apispec && go generate ./...

clean:
	docker compose down -v
