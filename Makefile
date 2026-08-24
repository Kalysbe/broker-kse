.PHONY: test api worker seed tidy

tidy:
	cd backend && go mod tidy

test:
	cd backend && go test ./...

api:
	cd backend && go run ./cmd/api

worker:
	cd backend && go run ./cmd/worker

seed:
	cd backend && go run ./cmd/seed

frontend:
	cd frontend && npm install && npm run dev
