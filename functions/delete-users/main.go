package main

import (
	"errors"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

type User struct {
	Email     string `json:"email"`
	User_ID   string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

var (
	ErrorInvalidUUID        = "invalid UUID"
	ErrorCouldNotDeleteItem = "could not delete item"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	id := request.QueryStringParameters["id"]
	role := request.QueryStringParameters["role"]
	region := os.Getenv("AWS_REGION")
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
		}, err
	}
	dynaClient := dynamodb.New(awsSession)
	cognitoClient := cognitoidentityprovider.New(awsSession)
	USER_TABLE := os.Getenv("USER_TABLE")
	if len(id) > 0 {
		res := DeleteUser(id, role, USER_TABLE, dynaClient)
		if res != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
			}, res
		}
		return events.APIGatewayProxyResponse{
			Body:       "Record successfully deleted",
			StatusCode: 200,
		}, nil
	}
	return events.APIGatewayProxyResponse{
		Body:       "Where the fuck is the id?",
		StatusCode: 404,
	}, errors.New(ErrorInvalidUUID)
}

func DeleteUser(id string, role string, tableName string, dynaClient dynamodbiface.DynamoDBAPI) error {
	
	input := &cognitoidentityprovider.AdminDeleteUserInput{
		"Username": aws.String(id),
		"UserPoolId": aws.String(role)
	}if err != nil {
		return errors.New(InternalErrorException)
	}
	
	input := &dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"user_id": {
				S: aws.String(id),
			},
		},
		TableName: aws.String(tableName),
	}
	_, err := dynaClient.DeleteItem(input)
	if err != nil {
		return errors.New(ErrorCouldNotDeleteItem)
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
