STACK_NAME ?= ascenda-serverless
GO := go
USER_FUNCTIONS := get-users create-users update-users delete-users
POINT_FUNCTIONS := get-points create-points update-points
MAKER_FUNCTIONS := get-makers get-checkers create-makers update-checkers
ROLE_FUNCTIONS := get-roles create-roles update-roles delete-roles
ADMINISTRATIVE_FUNCTIONS := get-logs lambda-authorizer
REGION := ap-southeast-1

build-user:
	${MAKE} ${MAKEOPTS} $(foreach userFunction,${USER_FUNCTIONS}, build-user-${userFunction})

build-user-%:
	cd functions/user/$* && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 ${GO} build -o bootstrap

build-point:
	${MAKE} ${MAKEOPTS} $(foreach pointFunction,${POINT_FUNCTIONS}, build-point-${pointFunction})

build-point-%:
	cd functions/point/$* && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 ${GO} build -o bootstrap

build-maker:
	${MAKE} ${MAKEOPTS} $(foreach makerFunction,${MAKER_FUNCTIONS}, build-maker-${makerFunction})

build-maker-%:
	cd functions/maker/$* && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 ${GO} build -o bootstrap

build-role:
	${MAKE} ${MAKEOPTS} $(foreach roleFunction,${ROLE_FUNCTIONS}, build-role-${roleFunction})

build-role-%:
	cd functions/role/$* && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 ${GO} build -o bootstrap

build-administrative:
	${MAKE} ${MAKEOPTS} $(foreach adminFunction,${ADMINISTRATIVE_FUNCTIONS}, build-administrative-${adminFunction})

build-administrative-%:
	cd functions/administrative/$* && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 ${GO} build -o bootstrap

start-local:
	@sam local start-api --env-vars env-vars.json

invoke-get:
	@sam local invoke --env-vars env-vars.json GetProductsFunction

clean:
	@rm $(foreach function,${USER_FUNCTIONS}, functions/user/${function}/bootstrap)
	@rm $(foreach function,${POINT_FUNCTIONS}, functions/point/${function}/bootstrap)
	@rm $(foreach function,${MAKER_FUNCTIONS}, functions/maker/${function}/bootstrap)
	@rm $(foreach function,${ROLE_FUNCTIONS}, functions/role/${function}/bootstrap)
	@rm $(foreach function,${ADMINISTRATIVE_FUNCTIONS}, functions/administrative/${function}/bootstrap)
deploy:
	@sam deploy --stack-name ${STACK_NAME};

deploy-auto: build-user build-point build-maker build-role build-administrative
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