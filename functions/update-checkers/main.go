package main

import (
	"encoding/json"
	"errors"
	"log"
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

var (
	ErrorCouldNotMarshalItem     = "could not marshal item"
	ErrorCouldNotDynamoPutItem   = "could not dynamo put item"
	ErrorCouldNotQueryDB         = "could not query db"
	ErrorInvalidMakerData        = "invalid maker data"
	ErrorInvalidMakerId          = "invalid maker id"
	ErrorInvalidPointsID         = "invalid points id"
	ErrorInvalidUserData         = "invalid user data"
	ErrorInvalidUserID           = "invalid user id"
	ErrorInvalidDecision         = "invalid decision"
	ErrorInvalidResourceType     = "resource type is invalid"
	ErrorUserDoesNotExist        = "target user does not exist"
	ErrorPointsDoesNotExist      = "target points does not exist"
	ErrorMakerDoesNotExist       = "target maker_id does not exist"
	ErrorFailedToUnmarshalRecord = "failed to unmarshal record"
	ErrorFailedToFetchRecord     = "failed to fetch record"
	ErrorFailedToFetchRecordID   = "failed to fetch record by uuid"
)

type DecisionBody struct {
	RequestId   string `json:"request_id"`
	CheckerRole string `json:"checker_role"`
	CheckerId   string `json:"checker_id"`
	Decision    string `json:"decision"`
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
	
	// unmarshal json body into DecisionBody
	var decisionBody DecisionBody

	if err := json.Unmarshal([]byte(request.Body), &decisionBody); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       string("Error in unmarshalling json body"),
		}, nil
	}
	
	if decisionBody.RequestId == "" || decisionBody.CheckerId == "" ||
		decisionBody.Decision == "" || decisionBody.CheckerRole == "" {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       string("Error missing json fields"),
		}, nil
	}
	
	//calling  to dynamo func
	res, err := MakerRequestDecision(decisionBody.RequestId, decisionBody.CheckerRole, decisionBody.CheckerId,
		decisionBody.Decision, MAKER_TABLE, USER_TABLE, POINTS_TABLE, request, dynaClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
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

func MakerRequestDecision(reqId, checkerRole, checkerUUID, decision, makerTableName, userTableName, pointsTableName string,
	req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) (
	[]utility.ReturnMakerRequest,
	error,
) {
	currentMakerRequest, err := FetchMakerRequestsByReqIdAndCheckerRole(reqId, checkerRole, makerTableName, req, dynaClient)
	if err != nil {
		if logErr := sendLogs(req, 2, 1, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, err
	}
	if len(currentMakerRequest) == 0 || len(currentMakerRequest[0].RequestUUID) == 0 {
		if logErr := sendLogs(req, 2, 3, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorMakerDoesNotExist)
	}
	
	if decision == "approve" {
		resourceType := currentMakerRequest[0].ResourceType

		// if maker request to change user table
		if resourceType == "user" {
			var userData User
			if err := json.Unmarshal(currentMakerRequest[0].RequestData, &userData); err != nil {
				if logErr := sendLogs(req, 2, 3, "maker", dynaClient, err); logErr != nil {
					log.Println("Logging err :", logErr)
				}
				return nil, errors.New(ErrorFailedToUnmarshalRecord)
			}
			
			_, err = FetchUserByID(userData.User_ID, req, userTableName, dynaClient)
			if err != nil {
				if logErr := sendLogs(req, 2, 1, "maker", dynaClient, err); logErr != nil {
					log.Println("Logging err :", logErr)
				}
				return nil, errors.New(ErrorUserDoesNotExist)
			}

			if len(userData.User_ID) == 0 {
				return nil, errors.New(ErrorInvalidUserID)
			}
			// make changes to user table
			_, err := UpdateUser(userData, req, userTableName, dynaClient)
			if err != nil {
				if logErr := sendLogs(req, 3, 3, "maker", dynaClient, err); logErr != nil {
					log.Println("Logging err :", logErr)
				}
				return nil, err
			}

			// if maker request to change points table
		} else if resourceType == "points" {
			var pointsData UserPoint
			if err := json.Unmarshal(currentMakerRequest[0].RequestData, &pointsData); err != nil {
				if logErr := sendLogs(req, 2, 3, "maker", dynaClient, err); logErr != nil {
					log.Println("Logging err :", logErr)
				}
				return nil, errors.New(ErrorCouldNotMarshalItem)
			}
			_, err = FetchUserPoint(pointsData.User_ID, req, pointsTableName, dynaClient)
			if err != nil {
				if logErr := sendLogs(req, 2, 1, "maker", dynaClient, err); logErr != nil {
					log.Println("Logging err :", logErr)
				}
				return nil, errors.New(ErrorPointsDoesNotExist)
			}

			// make changes to points table
			_, err := UpdateUserPoint(pointsData, req, pointsTableName, dynaClient)
			if err != nil {
				if logErr := sendLogs(req, 3, 3, "maker", dynaClient, err); logErr != nil {
					log.Println("Logging err :", logErr)
				}
				return nil, err
			}
		} else {
			if logErr := sendLogs(req, 2, 3, "maker", dynaClient, err); logErr != nil {
				log.Println("Logging err :", logErr)
			}
			return nil, errors.New(ErrorInvalidResourceType)
		}
		decision = "approved"
	} else if decision == "reject" {
		decision = "rejected"
	} else {
		if logErr := sendLogs(req, 2, 3, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorInvalidDecision)
	}

	makerRequests, err := FetchMakerRequest(reqId, makerTableName, req, dynaClient)
	if err != nil {
		if logErr := sendLogs(req, 2, 1, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, err
	}
	for i, request := range makerRequests {
		request.RequestStatus = decision
		request.CheckerUUID = checkerUUID
		
		makerRequests[i] = request
	}

	retMakerRequest, err := utility.BatchWriteToDynamoDB(len(makerRequests), makerRequests, makerTableName, dynaClient)
	if err != nil {
		if logErr := sendLogs(req, 3, 3, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, err
	}
	return retMakerRequest, nil
}

func FetchMakerRequest(requestID, tableName string, req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) ([]utility.MakerRequest, error) {
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("req_id = :req_id"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":req_id": {S: aws.String(requestID)},
		},
	}

	result, err := dynaClient.Query(queryInput)
	
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotQueryDB)
	}

	if len(result.Items) == 0 {
		if logErr := sendLogs(req, 3, 1, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorMakerDoesNotExist)
	}
	makerRequests := new([]utility.MakerRequest)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, makerRequests)
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	return *makerRequests, nil
}

func FetchMakerRequestsByReqIdAndCheckerRole(reqID, checkerRole, tableName string, req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) ([]utility.MakerRequest, error) {
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("req_id = :req_id AND checker_role = :checker_role"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":req_id":       {S: aws.String(reqID)},
			":checker_role": {S: aws.String(checkerRole)},
		},
	}

	result, err := dynaClient.Query(queryInput)
	
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotQueryDB)
	}

	makerRequests := new([]utility.MakerRequest)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, makerRequests)
	
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	if len(*makerRequests) == 0 {
		if logErr := sendLogs(req, 2, 1, "maker", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorMakerDoesNotExist)
	}

	return *makerRequests, nil
}

func UpdateUser(user User, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*User, error) {
	if user.User_ID == "" {
		err := errors.New(ErrorInvalidUserID)
		if logErr := sendLogs(req, 2, 3, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, err
	}

	//checking if user exist
	checkUser := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"user_id": {
				S: aws.String(user.User_ID),
			},
		},
		TableName: aws.String(tableName),
	}

	result, err := dynaClient.GetItem(checkUser)
	if err != nil {
		if logErr := sendLogs(req, 3, 1, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorFailedToFetchRecordID)
	}

	if result.Item == nil {
		return nil, errors.New("user does not exist")
	}

	av, err := dynamodbattribute.MarshalMap(user)
	if err != nil {
		if logErr := sendLogs(req, 3, 3, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = dynaClient.PutItem(input)
	if err != nil {
		if logErr := sendLogs(req, 3, 3, "user", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotDynamoPutItem)
	}

	if logErr := sendLogs(req, 1, 3, "user", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}

	return &user, nil
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

	if result.Item == nil {
		return nil, errors.New("user does not exist")
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

func UpdateUserPoint(userpoint UserPoint, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*UserPoint, error) {
	// check if points id is empty
	if userpoint.Points_ID == "" {
		err := errors.New(ErrorInvalidPointsID)
		if logErr := sendLogs(req, 2, 3, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, err
	}

	//checking if userpoint exist
	results, err := FetchUserPoint(userpoint.User_ID, req, tableName, dynaClient)
	if err != nil {
		if logErr := sendLogs(req, 2, 3, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorInvalidUserData)
	}

	var result = new(UserPoint)
	for _, v := range *results {
		if v.Points_ID == userpoint.Points_ID {
			result = &userpoint
		}
	}

	if result.Points_ID != userpoint.Points_ID {
		if logErr := sendLogs(req, 3, 3, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	av, err := dynamodbattribute.MarshalMap(result)
	if err != nil {
		if logErr := sendLogs(req, 3, 3, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	//updating user point in dynamo
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}
	_, err = dynaClient.PutItem(input)
	if err != nil {
		if logErr := sendLogs(req, 3, 3, "point", dynaClient, err); logErr != nil {
			log.Println("Logging err :", logErr)
		}
		return nil, errors.New(ErrorCouldNotDynamoPutItem)
	}

	if logErr := sendLogs(req, 1, 3, "point", dynaClient, err); logErr != nil {
		log.Println("Logging err :", logErr)
	}
	return result, nil
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

	if result.Items == nil {
		return nil, errors.New("user point does not exist")
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
