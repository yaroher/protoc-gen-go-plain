PROTO_REF_DIR=$(CURDIR)/goplain
PROTO_REF_FILES=$(shell find "$(PROTO_REF_DIR)" -type f -name '*.proto')
compile-proto-ref:
	protoc  --go_out=$(PROTO_REF_DIR) --go_opt=paths=source_relative --proto_path=$(PROTO_REF_DIR) $(PROTO_REF_FILES)

.PHONY: build
build: compile-proto-ref
	go build -o $(CURDIR)/bin/protoc-gen-go-plain ./

.PHONY: build-test
build-test: build
	mkdir -p $(CURDIR)/easyp_vendor/goplain
	cp $(CURDIR)/goplain/goplain.proto $(CURDIR)/easyp_vendor/goplain
	PATH=$(PATH):$(CURDIR)/bin LOG_LEVEL=debug LOG_FILE=$(CURDIR)/bin/protolog.txt easyp generate

.PHONY: build-test-protoc
build-test-protoc: build
	rm -f $(CURDIR)/bin/protolog.txt
	easyp mod vendor
	LOG_LEVEL=debug LOG_FILE=$(CURDIR)/bin/protolog.txt protoc \
		--plugin=protoc-gen-go-plain=$(CURDIR)/bin/protoc-gen-go-plain \
		--go_out=$(CURDIR) \
		--go_opt=paths=source_relative \
		--go-plain_out=$(CURDIR) \
		--go-plain_opt=paths=source_relative \
		--proto_path=$(CURDIR) \
		--proto_path=$(CURDIR)/easyp_vendor \
		$(CURDIR)/test/test.proto

branch=main
.PHONY: revision
revision: # Создание тега
	@if [ -e $(tag) ]; then \
		echo "error: Specify version 'tag='"; \
		exit 1; \
	fi
	git tag -d v${tag} || true
	git push --delete origin v${tag} || true
	git tag v$(tag)
	git push origin v$(tag)
