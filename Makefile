STACK_NAME ?= ascenda-serverless
FUNCTIONS := get-points get-logs get-roles get-makers get-checkers create-points create-roles create-makers update-points update-roles update-checkers delete-roles lambda-authorizer
GO := go
USER_FUNCTIONS := get-users create-users update-users delete-users
REGION := ap-southeast-1

build-user:
	${MAKE} ${MAKEOPTS} $(foreach userFunction,${USER_FUNCTIONS}, build-user-${userFunction})

build-user-%:
	cd functions/user/$* && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 ${GO} build -o bootstrap

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

deploy-auto: build build-user
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