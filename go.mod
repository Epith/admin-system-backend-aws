module ascenda

go 1.21

replace gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.2.8

require (
	github.com/aws/aws-lambda-go v1.41.0
	github.com/aws/aws-sdk-go v1.46.5
	github.com/google/uuid v1.4.0
)

require github.com/jmespath/go-jmespath v0.4.0 // indirect
