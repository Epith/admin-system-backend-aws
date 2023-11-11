package main

import (
	"ascenda/types"
	"ascenda/utility"
	"errors"
	"log"
	"os"
	"slices"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

var (
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorFailedToFetchRecord     = "failed to fetch record"
	ErrorFailedToFetchRecordID   = "failed to fetch record by uuid"
)

func handler(request events.APIGatewayV2CustomAuthorizerV2Request) (events.APIGatewayV2CustomAuthorizerSimpleResponse, error) {
	authorised := false
	accessToken := request.Headers["authorization"]
	route := request.RawPath[6:]
	method := request.RequestContext.HTTP.Method
	region := os.Getenv("AWS_REGION")
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)})
	if err != nil {
		return events.APIGatewayV2CustomAuthorizerSimpleResponse{
			IsAuthorized: false,
		}, nil
	}
	dynaClient := dynamodb.New(awsSession)
	cognitoClient := cognitoidentityprovider.New(awsSession)
	// Get the parameter value
	paramRole := "ROLES_TABLE"
	ROLES_TABLE := utility.GetParameterValue(awsSession, paramRole)
	//Check for user's role with cognito
	role, err := FetchUserAttributes(accessToken, cognitoClient)
	if err != nil {
		log.Println(err)
		return events.APIGatewayV2CustomAuthorizerSimpleResponse{
			IsAuthorized: false,
		}, nil
	}

	// Get list of access of Role
	access, err2 := GetAccessByRole(role, ROLES_TABLE, dynaClient)
	if err2 != nil {
		log.Println(err)
		return events.APIGatewayV2CustomAuthorizerSimpleResponse{
			IsAuthorized: false,
		}, nil
	}

	if access != nil {
		//Check Roles Item if Role provides permission
		authorised = slices.Contains(access.Access[route], method)
	}

	return events.APIGatewayV2CustomAuthorizerSimpleResponse{
		IsAuthorized: authorised,
	}, nil
}

func FetchUserAttributes(accessToken string, cognitoClient *cognitoidentityprovider.CognitoIdentityProvider) (string, error) {
	input := &cognitoidentityprovider.GetUserInput{
		AccessToken: &accessToken,
	}

	result, err := cognitoClient.GetUser(input)
	if err != nil {
		log.Println(err)
		return "", errors.New(ErrorFailedToFetchRecordID)
	}

	var role string
	for i := 0; i < len(result.UserAttributes); i++ {
		if *result.UserAttributes[i].Name == "custom:role" {
			role = *result.UserAttributes[i].Value
			break
		}
	}
	return role, nil
}

func GetAccessByRole(role, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*types.Role, error) {
	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"role": {
				S: aws.String(role),
			},
		},
		TableName: aws.String(tableName),
	}

	result, err := dynaClient.GetItem(input)
	if err != nil {
		return nil, errors.New(ErrorFailedToFetchRecordID)
	}

	item := new(types.Role)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}
	log.Println(item)
	return item, nil
}

func main() {
	lambda.Start(handler)
}
