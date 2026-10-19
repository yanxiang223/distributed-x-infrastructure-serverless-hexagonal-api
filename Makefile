STACK_NAME ?= distributed-x-infrastructure-serverless-hexagonal-api
FUNCTIONS := get-products get-product put-product delete-product
REGION ?= cn-north-4

# To try different version of Go
GO := go

# 函数包在 Linux 上运行，本地交叉编译产出可执行文件后打包上传。
# 编译产物名须与函数执行入口名称一致（本仓库统一为 handler）。
GOOS ?= linux
GOARCH ?= amd64

ci: build tests-unit

build:
		${MAKE} ${MAKEOPTS} $(foreach function,${FUNCTIONS}, build-${function})

build-%:
		cd functions/$* && GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 ${GO} build -o handler

package: build
		${MAKE} ${MAKEOPTS} $(foreach function,${FUNCTIONS}, package-${function})

package-%:
		cd functions/$* && rm -f function.zip && zip -q function.zip handler

clean:
		@rm -f $(foreach function,${FUNCTIONS}, functions/${function}/handler)
		@rm -f $(foreach function,${FUNCTIONS}, functions/${function}/function.zip)

# 查看到部署后的接口地址，用于联调与集成测试
api-url:
		@cd infra && terraform output -raw api_url

tests-unit:
		@go test -v -tags=unit -bench=. -benchmem -cover ./...

tests-integ:
		API_URL=$$(cd infra && terraform output -raw api_url) go test -v -tags=integration ./...

tests-load:
		API_URL=$$(cd infra && terraform output -raw api_url) artillery run load-testing/load-test.yml

deploy:
		cd infra && terraform init && terraform apply -auto-approve

destroy:
		cd infra && terraform destroy -auto-approve

# 函数工作流控制台提供「测试」功能，
# 可在函数详情页粘贴 functions/<name>/event.json 作为测试事件做单次调用。
invoke-hint:
		@echo "在函数工作流控制台的函数详情页选择「测试」，粘贴对应函数目录下的 event.json 作为测试事件。"

export GOBIN ?= $(shell pwd)/bin

STATICCHECK = $(GOBIN)/staticcheck

# Many Go tools take file globs or directories as arguments instead of packages
# go-runtime 为函数运行时 SDK 的本地副本（go.mod 中 replace 指向），不参与静态检查
GO_FILES := $(shell \
	       find . '(' -path '*/.*' -o -path './go-runtime' -o -path './bin' ')' -prune \
	       -o -name '*.go' -print | cut -b3-)

.PHONY: lint
lint: $(STATICCHECK)
		@rm -rf lint.log
		@echo "Checking formatting..."
		@gofmt -d -s $(GO_FILES) 2>&1 | tee lint.log
		@echo "Checking vet..."
		@go vet ./... 2>&1 | tee -a lint.log
		@echo "Checking staticcheck..."
		@$(STATICCHECK) ./... 2>&1 | tee -a lint.log
		@echo "Checking for unresolved FIXMEs..."
		@git grep -i fixme | grep -v -e Makefile | tee -a lint.log || true
		@[ ! -s lint.log ]
		@rm -f lint.log
		@echo "Checking 'go mod tidy'..."
		@make tidy
		@if ! git diff --quiet -- go.mod go.sum; then \
			echo "'go mod tidy' resulted in changes or working tree is dirty:"; \
			git --no-pager diff -- go.mod go.sum; \
			exit 1; \
		fi

$(STATICCHECK):
		cd tools && go install honnef.co/go/tools/cmd/staticcheck

.PHONY: tidy
tidy:
		@go mod tidy
