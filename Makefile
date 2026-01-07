PROTO_REF_DIR=$(CURDIR)/goplain
PROTO_REF_FILES=$(shell find "$(PROTO_REF_DIR)" -type f -name '*.proto')
compile-proto-ref:
	protoc  --go_out=$(PROTO_REF_DIR) --go_opt=paths=source_relative --proto_path=$(PROTO_REF_DIR) $(PROTO_REF_FILES)

.PHONY: build
build: compile-proto-ref
	go build ./

.PHONY: build-test
build-test: build
	rm -rf ./test/goplain
	LOG_LEVEL=debug LOG_FILE=$(CURDIR)/protolog.txt protoc \
		--plugin=./protoc-gen-go-plain \
		--go_out=./test \
		--go_opt=paths=source_relative \
		--go-plain_out=./test \
		--go-plain_opt=paths=source_relative \
		--proto_path=./test \
		--proto_path=. \
		test.proto ./goplain/goplain.proto
	go test ./...

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
