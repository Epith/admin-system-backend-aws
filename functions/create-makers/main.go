package main

import (
	"encoding/json"
	"errors"
	"log"
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

var (
	ErrorCouldNotMarshalItem     = "could not marshal item"
	ErrorCouldNotDynamoPutItem   = "could not dynamo put item"
	ErrorInvalidMakerData        = "invalid maker data"
	ErrorInvalidPointsID         = "invalid points id"
	ErrorInvalidResourceType     = "resource type is invalid"
	ErrorUserDoesNotExist        = "target user does not exist"
	ErrorPointsDoesNotExist      = "target points does not exist"
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorFailedToFetchRecord     = "failed to fetch record"
	ErrorFailedToFetchRecordID   = "failed to fetch record by uuid"
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
type UserPoint struct {
	User_ID   string `json:"user_id"`
	Points_ID string `json:"points_id"`
	Points    int    `json:"points"`
}

type User struct {
	Email     string `json:"email"`
	User_ID   string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
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

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//getting variables
	region := os.Getenv("AWS_REGION")
	USER_TABLE := os.Getenv("USER_TABLE")
	POINTS_TABLE := os.Getenv("POINTS_TABLE")
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

	//calling create maker request to dynamo func
	res, err := CreateMakerRequest(request, MAKER_TABLE, USER_TABLE, POINTS_TABLE, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string(err.Error()),
		}, nil
	}

	body, _ := json.Marshal(res)

	stringBody := string(body)

	return events.APIGatewayProxyResponse{
		Body:       stringBody,
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func CreateMakerRequest(req events.APIGatewayProxyRequest, makerTableName, userTableName, pointsTableName string, dynaClient dynamodbiface.DynamoDBAPI) (
	[]ReturnMakerRequest, error) {
	var postMakerRequest NewMakerRequest

	//marshall body to maker request struct
	if err := json.Unmarshal([]byte(req.Body), &postMakerRequest); err != nil {
		return nil, errors.New(ErrorInvalidMakerData)
	}
	
	if postMakerRequest.MakerUUID == "" {
		return nil, errors.New(ErrorInvalidMakerData)
	}
	
	_, err := FetchUserByID(postMakerRequest.MakerUUID, req, userTableName, dynaClient)
	if err != nil {
		return nil, errors.New(ErrorUserDoesNotExist)
	}
	
	if postMakerRequest.ResourceType == "user" {
		//marshall body to point struct
		var userData User
		if err := json.Unmarshal(postMakerRequest.RequestData, &userData); err != nil {
			return nil, errors.New(ErrorCouldNotMarshalItem)
		}
		
		// check if user exist
		_, err = FetchUserByID(userData.User_ID, req, userTableName, dynaClient)
		if err != nil {
			return nil, errors.New(userData.User_ID)
		}
		makerRequests := DeconstructPostMakerRequest(postMakerRequest)
		roleCount := len(postMakerRequest.CheckerRole)
		return BatchWriteToDynamoDB(roleCount, makerRequests, makerTableName, dynaClient)

	} else if postMakerRequest.ResourceType == "points" {

		//marshall body to point struct
		var pointsData UserPoint
		if err := json.Unmarshal(postMakerRequest.RequestData, &pointsData); err != nil {
			return nil, errors.New(ErrorCouldNotMarshalItem)
		}
		// check if points exist
		_, err = FetchUserPoint(pointsData.User_ID, req, pointsTableName, dynaClient)
		if err != nil {
			return nil, errors.New(ErrorPointsDoesNotExist)
		}

		if pointsData.Points_ID == "" {
			return nil, errors.New(ErrorInvalidPointsID)
		}

		makerRequests := DeconstructPostMakerRequest(postMakerRequest)
		roleCount := len(postMakerRequest.CheckerRole)
		return BatchWriteToDynamoDB(roleCount, makerRequests, makerTableName, dynaClient)
	}

	return nil, errors.New(ErrorInvalidResourceType)
}

func FetchUserByID(id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*User, error) {
	//get single user from dynamo
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
		if logErr := sendLogs(req, 3, 1, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToFetchRecordID)
	}
	item := new(User)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}
	
	if logErr := sendLogs(req, 1, 1, "user", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}
	return item, nil
}

func FetchUserPoint(user_id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*[]UserPoint, error) {
	//getting single single user point
	input := &dynamodb.QueryInput{
		TableName: aws.String(tableName),
		KeyConditions: map[string]*dynamodb.Condition{
			"user_id": {
				ComparisonOperator: aws.String("EQ"),
				AttributeValueList: []*dynamodb.AttributeValue{
					{
						S: aws.String(user_id),
					},
				},
			},
		},
	}

	result, err := dynaClient.Query(input)
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToFetchRecord)
	}

	item := new([]UserPoint)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, item)
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToUnmarshalRecord)
	}

	if logErr := sendLogs(req, 1, 1, "point", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return item, nil
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
