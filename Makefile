MODULES := log i18n httpx config db identity auth job rate_limit productmatch currency exchange apperr audit billing featureflags

.PHONY: lint test tidy sync tools

lint: ## golangci-lint per module (root has go.work only, no go.mod)
	@command -v golangci-lint >/dev/null || { \
		echo "golangci-lint not found. Run: make tools"; \
		exit 1; \
	}
	@for m in $(MODULES); do \
		echo "==> lint $$m"; \
		(cd $$m && golangci-lint run --config ../.golangci.yml ./...) || exit 1; \
	done

test: sync ## go test ./... for each module
	@for m in $(MODULES); do \
		echo "==> test $$m"; \
		(cd $$m && go test ./... -count=1) || exit 1; \
	done

sync: ## go work sync
	go work sync

tidy: sync ## go mod tidy in all modules
	@for m in $(MODULES); do \
		echo "==> tidy $$m"; \
		(cd $$m && go mod tidy) || exit 1; \
	done

tools: ## install dev tools (golangci-lint v2)
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
