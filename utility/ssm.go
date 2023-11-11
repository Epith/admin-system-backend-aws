package utility

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

func GetParameterValue(session *session.Session, paramName string) (*ssm.GetParameterOutput, error) {
	// Create an SSM client
	svc := ssm.New(session)
	// Get the parameter value
	paramValue, err := svc.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		log.Fatal(err)
	}

	return paramValue, err
}
