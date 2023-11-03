package main

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"time"

	"ascenda/functions/utility"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/google/uuid"
)

type Log struct {
	Log_ID          string      `json:"log_id"`
	Severity        int         `json:"severity"`
	User_ID         string      `json:"user_id"`
	Action_Type     int         `json:"action_type"`
	Resource_Type   string      `json:"resource_type"`
	Body            interface{} `json:"body"`
	QueryParameters interface{} `json:"query_parameters"`
	Error           interface{} `json:"error"`
	Timestamp       time.Time   `json:"timestamp"`
}

var (
	ErrorCouldNotMarshalItem = "could not marshal item"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//get variables
	status := request.QueryStringParameters["status"]
	role := request.QueryStringParameters["role"]
	region := os.Getenv("AWS_REGION")
	MAKER_TABLE := os.Getenv("MAKER_TABLE")

	//setting up dynamo session
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error setting up aws session"),
		}, err
	}
	dynaClient := dynamodb.New(awsSession)

	// filter by client role and maker request status
	if len(status) > 0 {
		res, err := FetchMakerRequestsByCheckerRoleAndStatus(role, status, MAKER_TABLE, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string(err.Error()),
			}, err
		}
		stringBody, _ := json.Marshal(res)
		return events.APIGatewayProxyResponse{
			Body:       string(stringBody),
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}
	return events.APIGatewayProxyResponse{
		StatusCode: 400,
		Body:       string("missing query parameter"),
	}, err
}

func FetchMakerRequestsByCheckerRoleAndStatus(checker_role, requestStatus, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*[]utility.MakerRequest, error) {
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("checker_role-request_status-index"),
		KeyConditionExpression: aws.String("#checker_role = :checker_role AND #request_status = :request_status"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":checker_role":   {S: aws.String(checker_role)},
			":request_status": {S: aws.String(requestStatus)},
		},
		ExpressionAttributeNames: map[string]*string{
			"#checker_role":   aws.String("checker_role"),
			"#request_status": aws.String("request_status"),
		},
	}

	result, err := dynaClient.Query(queryInput)
	if err != nil {
		return nil, err
	}

	makerRequests := new([]utility.MakerRequest)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, makerRequests)
	if err != nil {
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	return makerRequests, nil
}

func main() {
	lambda.Start(handler)
}

func sendLogs(req events.APIGatewayProxyRequest, severity int, action int, resource string, dynaClient dynamodbiface.DynamoDBAPI, err error) error {
	LOGS_TABLE := os.Getenv("LOGS_TABLE")
	//create log struct
	log := Log{}
	log.Body = RemoveNewlineAndUnnecessaryWhitespace(req.Body)
	log.QueryParameters = req.QueryStringParameters
	log.Error = err
	log.Log_ID = uuid.NewString()
	log.Severity = severity
	log.User_ID = req.RequestContext.Identity.User
	log.Action_Type = action
	log.Resource_Type = resource
	log.Timestamp = time.Now().UTC()

	av, err := dynamodbattribute.MarshalMap(log)

	if err != nil {
		return errors.New("failed to marshal log")
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(LOGS_TABLE),
	}
	_, err = dynaClient.PutItem(input)
	if err != nil {
		return errors.New("Could not dynamo put")
	}
	return nil
}

func RemoveNewlineAndUnnecessaryWhitespace(body string) string {
	// Remove newline characters
	body = regexp.MustCompile(`\n|\r`).ReplaceAllString(body, "")

	// Remove unnecessary whitespace
	body = regexp.MustCompile(`\s{2,}|\t`).ReplaceAllString(body, " ")

	// Remove the character `\"`
	body = regexp.MustCompile(`\"`).ReplaceAllString(body, "")

	// Trim the body
	body = strings.TrimSpace(body)

	return body
}
