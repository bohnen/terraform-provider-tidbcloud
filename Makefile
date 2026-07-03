default: testacc

# Run acceptance tests
.PHONY: testacc
testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

.PHONY: generate-mocks
generate-mocks: ## Generate mock objects
	@echo "==> Generating mock objects"
	go install github.com/golang/mock/mockgen@v1.6.0
	# mockgen --source=./tidbcloud/api_client.go --destination=./mock/mock_client.go --package mock
	mockgen --source=./tidbcloud/serverless_api_client.go --destination=./mock/mock_serverless_client.go --package mock
	mockgen --source=./tidbcloud/iam_api_client.go --destination=./mock/mock_iam_client.go --package mock
	mockgen --source=./tidbcloud/dedicated_api_client.go --destination=./mock/mock_dedicated_client.go --package mock
	

.PHONY: generate-dedicated-import-client
generate-dedicated-import-client: ## Regenerate the dedicated import API client from the checked-in swagger subset
	@echo "==> Generating dedicated import client (requires openapi-generator-cli 7.12.0, same as tidbcloud-cli SDK)"
	openapi-generator-cli generate \
		-i pkg/tidbcloud/v1beta1/dedicated/imp/dedicated-import.swagger.json \
		-g go \
		-o pkg/tidbcloud/v1beta1/dedicated/imp \
		--package-name imp \
		--global-property apis,models,supportingFiles \
		--additional-properties=enumClassPrefix=true,isGoSubmodule=false,withGoMod=false,generateInterfaces=false
	rm -rf pkg/tidbcloud/v1beta1/dedicated/imp/git_push.sh \
		pkg/tidbcloud/v1beta1/dedicated/imp/test \
		pkg/tidbcloud/v1beta1/dedicated/imp/docs \
		pkg/tidbcloud/v1beta1/dedicated/imp/api \
		pkg/tidbcloud/v1beta1/dedicated/imp/.openapi-generator \
		pkg/tidbcloud/v1beta1/dedicated/imp/.openapi-generator-ignore \
		pkg/tidbcloud/v1beta1/dedicated/imp/.travis.yml \
		pkg/tidbcloud/v1beta1/dedicated/imp/.gitignore
	gofmt -w pkg/tidbcloud/v1beta1/dedicated/imp
