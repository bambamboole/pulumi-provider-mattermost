BIN     := bin/pulumi-resource-mattermost
VERSION ?= $(shell node -p "require('./pulumi-plugin.json').version")
LDFLAGS := -X main.version=$(VERSION)
SDK_DIR := sdk/nodejs

.PHONY: build test vet tidy schema gen-sdk build-sdk install-local clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/pulumi-resource-mattermost

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

schema: build
	pulumi package get-schema ./$(BIN) > schema.json

gen-sdk: build
	rm -rf $(SDK_DIR)
	pulumi package gen-sdk ./$(BIN) --language nodejs --out sdk
	cp LICENSE $(SDK_DIR)/LICENSE 2>/dev/null || true
	node patch-sdk.js
	cd $(SDK_DIR) && npm install --package-lock-only --ignore-scripts --no-audit --no-fund

build-sdk:
	cd $(SDK_DIR) && npm ci --no-audit --no-fund && npm run build
	cp $(SDK_DIR)/package.json $(SDK_DIR)/README.md $(SDK_DIR)/bin/
	if [ -f LICENSE ]; then cp LICENSE $(SDK_DIR)/bin/; fi
	cd $(SDK_DIR)/bin && node -e "const fs=require('fs');const p=JSON.parse(fs.readFileSync('package.json'));p.version='$(VERSION)';p.pulumi.version='$(VERSION)';delete p.scripts.prepare;delete p.devDependencies;fs.writeFileSync('package.json',JSON.stringify(p,null,2)+'\n')"

install-local: build
	@pulumi plugin rm resource mattermost $(VERSION) --yes 2>/dev/null || true
	rm -rf bin/plugin
	mkdir -p bin/plugin
	cp $(BIN) bin/plugin/pulumi-resource-mattermost
	cp pulumi-plugin.json bin/plugin/pulumi-plugin.json
	tar czf bin/pulumi-resource-mattermost-$(VERSION).tar.gz -C bin/plugin pulumi-resource-mattermost pulumi-plugin.json
	pulumi plugin install resource mattermost $(VERSION) --file bin/pulumi-resource-mattermost-$(VERSION).tar.gz

clean:
	rm -rf bin sdk schema.json
