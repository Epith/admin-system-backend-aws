package main

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/google/uuid"
)

type MakerRequest struct {
	RequestUUID   string          `json:"req_id"`
	CheckerRole   string          `json:"checker_role"`
	MakerUUID     string          `json:"maker_id"`
	CheckerUUID   string          `json:"checker_id"`
	RequestStatus string          `json:"request_status"`
	ResourceType  string          `json:"resource_type"`
	RequestData   json.RawMessage `json:"request_data"`
}

type ReturnMakerRequest struct {
	RequestUUID   string          `json:"req_id"`
	CheckerRole   []string        `json:"checker_role"`
	MakerUUID     string          `json:"maker_id"`
	CheckerUUID   string          `json:"checker_id"`
	RequestStatus string          `json:"request_status"`
	ResourceType  string          `json:"resource_type"`
	RequestData   json.RawMessage `json:"request_data"`
}

type NewMakerRequest struct {
	CheckerRole  []string        `json:"checker_roles"`
	MakerUUID    string          `json:"maker_id"`
	ResourceType string          `json:"resource_type"`
	RequestData  json.RawMessage `json:"request_data"`
}

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
	ErrorFailedToFetchRecord   = "failed to fetch record"
	ErrorCouldNotMarshalItem   = "could not marshal item"
	ErrorMakerDoesNotExist     = "request.maker_id does not exist"
	ErrorCouldNotDynamoPutItem = "could not dynamo put item"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//get variables
	req_id := request.QueryStringParameters["req_id"]
	region := os.Getenv("AWS_REGION")
	MAKER_TABLE := os.Getenv("MAKER_TABLE")

	//setting up dynamo session
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(region)})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error setting up aws session"),
		}, nil
	}
	dynaClient := dynamodb.New(awsSession)

	// get by req id
	if len(req_id) > 0 {
		res, err := FetchMakerRequest(req_id, MAKER_TABLE, request, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string(err.Error()),
			}, nil
		}
		stringBody, _ := json.Marshal(res)
		return events.APIGatewayProxyResponse{
			Body:       string(stringBody),
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}

	// get by maker id and status
	makerId := request.QueryStringParameters["maker_id"]
	status := request.QueryStringParameters["status"]
	if len(makerId) > 0 && len(status) > 0 {
		res, err := FetchMakerRequestsByMakerIdAndStatus(makerId, status, MAKER_TABLE, request, dynaClient)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       string(err.Error()),
			}, nil
		}
		stringBody, _ := json.Marshal(res)
		return events.APIGatewayProxyResponse{
			Body:       string(stringBody),
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}
	// get all
	res, err := FetchMakerRequests(MAKER_TABLE, request, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string(err.Error()),
		}, nil
	}

	body, _ := json.Marshal(res)
	stringBody := string(body)
	return events.APIGatewayProxyResponse{
		Body:       string(stringBody),
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func FetchMakerRequest(requestID, tableName string, req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) ([]ReturnMakerRequest, error) {
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("req_id = :req_id"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":req_id": {S: aws.String(requestID)},
		},
	}

	result, err := dynaClient.Query(queryInput)
	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, errors.New(ErrorMakerDoesNotExist)
	}

	makerRequests := new([]MakerRequest)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, makerRequests)
	if err != nil {
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	return FormatMakerRequest(*makerRequests), nil

}

func FetchMakerRequests(tableName string, req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) ([]ReturnMakerRequest, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int64(int64(3000)),
	}

	result, err := dynaClient.Scan(input)
	if err != nil {
		return nil, errors.New(ErrorFailedToFetchRecord)
	}
	item := new([]MakerRequest)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, item)
	return FormatMakerRequest(*item), nil
}

func FetchMakerRequestsByMakerIdAndStatus(makerID, requestStatus, tableName string, req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) ([]ReturnMakerRequest, error) {
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("maker_id-request_status-index"),
		KeyConditionExpression: aws.String("#maker_id = :maker_id AND #request_status = :request_status"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":maker_id":       {S: aws.String(makerID)},
			":request_status": {S: aws.String(requestStatus)},
		},
		ExpressionAttributeNames: map[string]*string{
			"#maker_id":       aws.String("maker_id"),
			"#request_status": aws.String("request_status"),
		},
	}

	result, err := dynaClient.Query(queryInput)
	if err != nil {
		return nil, err
	}

	makerRequests := new([]MakerRequest)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, makerRequests)
	if err != nil {
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	return FormatMakerRequest(*makerRequests), nil
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

func BatchWriteToDynamoDB(roleCount int, makerRequests []MakerRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) ([]ReturnMakerRequest, error) {
	writeRequests := make([]*dynamodb.WriteRequest, roleCount)

	for i, request := range makerRequests {
		item, err := dynamodbattribute.MarshalMap(request)
		if err != nil {
			return nil, err
		}

		writeRequest := &dynamodb.WriteRequest{
			PutRequest: &dynamodb.PutRequest{
				Item: item,
			},
		}

		writeRequests[i] = writeRequest
	}

	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]*dynamodb.WriteRequest{
			tableName: writeRequests,
		},
	}
	_, err := dynaClient.BatchWriteItem(input)

	if err != nil {
		return nil, errors.New(ErrorCouldNotDynamoPutItem)
	}
	return FormatMakerRequest(makerRequests), nil
}

func FormatMakerRequest(makerRequests []MakerRequest) []ReturnMakerRequest {
	makerRequestsMap := make(map[string]ReturnMakerRequest)
	for _, request := range makerRequests {
		resRequest := makerRequestsMap[request.RequestUUID]
		if resRequest.RequestUUID == "" {
			makerRequestsMap[request.RequestUUID] = ReturnMakerRequest{
				RequestUUID:   request.RequestUUID,
				CheckerRole:   []string{request.CheckerRole},
				MakerUUID:     request.MakerUUID,
				CheckerUUID:   request.CheckerUUID,
				RequestStatus: request.RequestStatus,
				ResourceType:  request.ResourceType,
				RequestData:   request.RequestData,
			}
		} else {
			resRequest.CheckerRole = append(resRequest.CheckerRole, request.CheckerRole)
			makerRequestsMap[request.RequestUUID] = resRequest
		}
	}
	retRequests := make([]ReturnMakerRequest, 0, len(makerRequestsMap))
	for _, value := range makerRequestsMap {
		retRequests = append(retRequests, value)
	}

	return retRequests
}

func DeconstructPostMakerRequest(postMakerRequest NewMakerRequest) []MakerRequest {
	roleCount := len(postMakerRequest.CheckerRole)
	makerRequests := make([]MakerRequest, roleCount)
	reqId := uuid.NewString()

	for i := 0; i < roleCount; i++ {
		var makerRequest MakerRequest

		makerRequest.RequestUUID = reqId
		makerRequest.RequestStatus = "pending"
		makerRequest.CheckerUUID = ""
		makerRequest.CheckerRole = postMakerRequest.CheckerRole[i]
		makerRequest.MakerUUID = postMakerRequest.MakerUUID
		makerRequest.ResourceType = postMakerRequest.ResourceType
		makerRequest.RequestData = postMakerRequest.RequestData

		makerRequests[i] = makerRequest
	}
	return makerRequests
}
