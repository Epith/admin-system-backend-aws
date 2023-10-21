STACK_NAME ?= ascenda-serverless
FUNCTIONS := users points
GO := go

build:
		${MAKE} ${MAKEOPTS} $(foreach function,${FUNCTIONS}, build-${function})

build-%:
		cd functions/$* && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 ${GO} build -o bootstrap

star-local:
	@sam local start-api --env-vars env-vars.json

invoke:
	@sam local invoke --env-vars env-vars.json GetProductsFunction

invoke-put:
	@sam local invoke --env-vars env-vars.json --event functions/put-product/event.json PutProductFunction

invoke-get:
	@sam local invoke --env-vars env-vars.json --event functions/get-product/event.json GetProductFunction

invoke-delete:
	@sam local invoke --env-vars env-vars.json --event functions/delete-product/event.json DeleteProductFunction

invoke-stream:
	@sam local invoke --env-vars env-vars.json --event functions/products-stream/event.json DDBStreamsFunction

clean:
	@rm $(foreach function,${FUNCTIONS}, functions/${function}/bootstrap)

deploy:
	if [ -f samconfig.toml ]; \
		then sam deploy --stack-name ${STACK_NAME}; \
		else sam deploy -g --stack-name ${STACK_NAME}; \
  fi

delete:
	@sam delete --stack-name ${STACK_NAME}

.PHONY: tidy
tidy:
	@$(foreach dir,$(MODULE_DIRS),(cd $(dir) && go mod tidy) &&) true

load-test:
	API_URL=$$(aws cloudformation describe-stacks --stack-name $(STACK_NAME) \
	  --region $(REGION) \
		--query 'Stacks[0].Outputs[?OutputKey==`ApiUrl`].OutputValue' \
		--output text) artillery run load-testing/load-test.yml