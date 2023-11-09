STACK_NAME ?= ascenda-serverless
FUNCTIONS := get-users get-points get-logs get-roles get-makers get-checkers create-users create-points create-roles create-makers update-points update-users update-roles update-checkers delete-users delete-roles lambda-authorizer
GO := go
REGION := ap-southeast-1

build:
	${MAKE} ${MAKEOPTS} $(foreach function,${FUNCTIONS}, build-${function})

build-%:
	cd functions/$* && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 ${GO} build -o bootstrap

start-local:
	@sam local start-api --env-vars env-vars.json

invoke-get:
	@sam local invoke --env-vars env-vars.json GetProductsFunction

clean:
	@rm $(foreach function,${FUNCTIONS}, functions/${function}/bootstrap)

deploy:
	@sam deploy --stack-name ${STACK_NAME};

deploy-auto: build
	@sam deploy --stack-name ${STACK_NAME} --no-confirm-changeset --no-fail-on-empty-changeset;

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