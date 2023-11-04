package main

import (
	"errors"
	"os"
	"slices"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

var (
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorFailedToFetchRecord     = "failed to fetch record"
	ErrorFailedToFetchRecordID   = "failed to fetch record by uuid"
)

type User struct {
	Email     string `json:"email"`
	User_ID   string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

type Role struct {
	Role   string              `json:"role"`
	Access map[string][]string `json:"access"`
}

func handler(request events.APIGatewayV2CustomAuthorizerV2Request) events.APIGatewayV2CustomAuthorizerSimpleResponse {

	authorised := false
	role := request.Headers["Role"]
	id := request.IdentitySource[0]
	route := request.RouteKey
	method := request.RequestContext.HTTP.Method
	region := os.Getenv("AWS_REGION")
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)})

	if err != nil {
		return events.APIGatewayV2CustomAuthorizerSimpleResponse{
			IsAuthorized: false,
		}
	}

	dynaClient := dynamodb.New(awsSession)
	USER_TABLE := os.Getenv("USER_TABLE")
	ROLE_TABLE := os.Getenv("ROLES_TABLE")

	//Check User Table if role exist?
	item, err := FetchUserByID(id, USER_TABLE, dynaClient)
	if err == nil && role == item.Role {
		// Get list of access of Role
		access, err2 := GetAccessByRole(role, ROLE_TABLE, dynaClient)
		if err2 == nil {
			//Check Roles Item if Role provides permission
			authorised = slices.Contains(access.Access[route], method)
		}
	}

	return events.APIGatewayV2CustomAuthorizerSimpleResponse{
		IsAuthorized: authorised,
	}
}

func FetchUserByID(id, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*User, error) {
	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"user_id": {
				S: aws.String(id),
			},
		},
		TableName: aws.String(tableName),
	}

	result, err := dynaClient.GetItem(input)
	if err != nil {
		return nil, errors.New(ErrorFailedToFetchRecordID)
	}

	item := new(User)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}
	return item, nil
}

func GetAccessByRole(role, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*Role, error) {
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

	item := new(Role)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}

	return item, nil
}
