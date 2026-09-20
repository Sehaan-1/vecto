.PHONY: all clean codegen test-go test-py test-web build-go build-web package

all: package

clean:
	@rm -rf dist generated

codegen: schema/api.json scripts/generate_routes.py
	@python scripts/generate_routes.py

test-go: codegen services/api/math.go services/api/math_test.go
	@go test ./services/api/...

test-py: scripts/data_processor.py scripts/test_processor.py
	@python scripts/test_processor.py

test-web: web/index.js web/index.test.js
	@node web/index.test.js

build-go: test-go services/api/main.go
	@go run services/api/main.go build

build-web: test-web web/index.js scripts/build_bundle.py
	@python scripts/build_bundle.py

package: build-go build-web test-py scripts/package_release.py
	@python scripts/package_release.py
