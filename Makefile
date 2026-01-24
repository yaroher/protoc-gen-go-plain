PROTO_REF_DIR=$(CURDIR)/goplain
PROTO_REF_FILES=$(shell find "$(PROTO_REF_DIR)" -type f -name '*.proto')
compile-proto-ref:
	protoc  --go_out=$(PROTO_REF_DIR) --go_opt=paths=source_relative --proto_path=$(PROTO_REF_DIR) $(PROTO_REF_FILES)

.PHONY: build
build: compile-proto-ref
	go build -o $(CURDIR)/bin/protoc-gen-go-plain ./

.PHONY: run-test
run-test:
	go clean -testcache && go test -v ./...

.PHONY: run-bench
run-bench:
	go clean -testcache && go test -bench=. -v ./...

.PHONY: .clean-test-nda
.clean-test-nda:
	find ./test/nda/xdr -type f -name "*.go" -delete

NDA_PROTO_DIR=$(CURDIR)/test/nda
NDA_PROTO_FILES=$(shell find "$(NDA_PROTO_DIR)" -type f -name '*.proto')
.PHONY: build-test-nda
build-test-nda: build .clean-test-nda
	rm -f $(CURDIR)/bin/protolog.txt
	LOG_LEVEL=debug LOG_FILE=$(CURDIR)/bin/protolog.txt protoc \
		--plugin=protoc-gen-go-plain=$(CURDIR)/bin/protoc-gen-go-plain \
		--go_out=$(CURDIR) \
		--go_opt=paths=source_relative \
		--go-plain_out=$(CURDIR) \
		--go-plain_opt=paths=source_relative,json_jx=true \
		--proto_path=$(CURDIR) \
		$(NDA_PROTO_FILES)
	sed -i 's/\\n/\n/g' $(CURDIR)/bin/protolog.txt
	sed -i 's/\\t/\t/g' $(CURDIR)/bin/protolog.txt
	sed -i 's/\\//g' $(CURDIR)/bin/protolog.txt
	sed -i 's/[[:space:]]\+/ /g'  $(CURDIR)/bin/protolog.txt

.PHONY: run-test-nda
run-test-nda:
	go clean -testcache && go test -v ./test/nda/...

.PHONY: bench-test-nda
bench-test-nda:
	go clean -testcache && go test -bench=. -v ./test/nda/...


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
