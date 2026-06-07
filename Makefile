.PHONY: dev run test build clean

dev:  ## Start both backend and frontend in dev mode
	@cd server && go run ./cmd/chatd & \
	 cd client && npm run dev

run:   ## Start the Go server (serve built frontend)
	cd server && go run ./cmd/chatd

test:  ## Run all tests
	cd server && go test ./... -cover -count=1 -timeout 60s

build: ## Build everything
	cd client && npm ci && npm run build
	cd server && go build -o ../chatd ./cmd/chatd/

clean:
	rm -f chatd server/chat.db server/chat.db-*
