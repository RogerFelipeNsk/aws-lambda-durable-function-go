.PHONY: test test-go build-lambda build-frontend local frontend-dev deploy fmt tidy

GO ?= go
export CGO_ENABLED=0
export AWS_REGION ?= us-east-2

test: test-go

test-go:
	cd backend && $(GO) test ./...

tidy:
	cd backend && $(GO) mod tidy

fmt:
	cd backend && $(GO) fmt ./...

build-lambda:
	mkdir -p dist/api dist/durable
	cd backend && GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags="-s -w" -o ../dist/api/bootstrap ./cmd/api
	cd backend && GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags="-s -w" -o ../dist/durable/bootstrap ./cmd/durable

local:
	cd backend && $(GO) run ./cmd/api

frontend-dev:
	cd frontend && npm install && npm run dev

build-frontend:
	cd frontend && npm install && npm run build

deploy: test-go build-lambda
	cd infra && tofu init -input=false
	cd infra && tofu apply -input=false -auto-approve
	$(MAKE) build-frontend
	@bucket=$$(cd infra && tofu output -raw frontend_bucket); \
	dist=$$(cd infra && tofu output -raw cloudfront_distribution_id); \
	aws s3 sync frontend/dist "s3://$$bucket" --delete; \
	aws cloudfront create-invalidation --distribution-id "$$dist" --paths "/*" >/dev/null; \
	echo "Loja: $$(cd infra && tofu output -raw cloudfront_url)"
