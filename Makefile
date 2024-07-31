BUILD_DIR 		?= $(abspath build)
GET_GOARCH 		 = $(word 2,$(subst -, ,$1))
GET_GOOS   		 = $(word 1,$(subst -, ,$1))
GOBUILD   		?= $(shell go env GOOS)-$(shell go env GOARCH)
GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
VETARGS? 		 =-all

$(BUILD_DIR):
	mkdir -p $@

.PHONY: install
install:
	go get github.com/kardianos/govendor
	govendor sync

.PHONY: build
build:
	go mod vendor
	go mod tidy
	go install

.PHONY: fmt
fmt:
	@echo "=> Running go fmt" ;
	@if [ -n "`go fmt ${GOFILES_NOVENDOR}`" ]; then \
		echo "[ERR] go fmt updated formatting. Please commit formatted code first."; \
		exit 1; \
	fi

.PHONY: vet
vet: fmt
	@go vet 2>/dev/null ; if [ $$? -eq 3 ]; then \
		go get golang.org/x/tools/cmd/vet; \
	fi

	@echo "=> Running go vet $(VETARGS) ${GOFILES_NOVENDOR}"
	@go vet $(VETARGS) ${GOFILES_NOVENDOR} ; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "[LINT] Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
	fi

BINARIES = $(addprefix $(BUILD_DIR)/aws-dynamic-consul-catalog-, $(GOBUILD))
$(BINARIES): $(BUILD_DIR)/aws-dynamic-consul-catalog-%: $(BUILD_DIR)
	@echo "=> building $@ ..."
	GOOS=$(call GET_GOOS,$*) GOARCH=$(call GET_GOARCH,$*) CGO_ENABLED=0 go build -o $@

.PHONY: dist
dist: fmt vet
	@echo "=> building ..."
	$(MAKE) -j $(BINARIES)

.PHONY: docker
docker:
	@echo "=> build and push Docker image ..."
	docker build -f Dockerfile -t seatgeek/aws-dynamic-consul-catalog:$GIT_COMMIT_SHA .
	docker tag seatgeek/aws-dynamic-consul-catalog:$GIT_COMMIT_SHA seatgeek/aws-dynamic-consul-catalog:v1.8.0
	docker push seatgeek/aws-dynamic-consul-catalog:v1.8.0
