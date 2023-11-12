package main

import (
	"encoding/json"
	"errors"
	"os"

	"ascenda/types"
	"ascenda/utility"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//getting variables
	region := os.Getenv("AWS_REGION")

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

	// Get the parameter value
	paramUser := "USER_TABLE"
	outputUser, err := utility.GetParameterValue(awsSession, paramUser)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting user table parameter store"),
		}, nil
	}
	USER_TABLE := *outputUser.Parameter.Value

	paramPoints := "POINTS_TABLE"
	outputPoints, err := utility.GetParameterValue(awsSession, paramPoints)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting points table parameter store"),
		}, nil
	}
	POINTS_TABLE := *outputPoints.Parameter.Value

	paramMaker := "MAKER_TABLE"
	outputMaker, err := utility.GetParameterValue(awsSession, paramMaker)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       string("Error getting maker table parameter store"),
		}, nil
	}
	MAKER_TABLE := *outputMaker.Parameter.Value

	// unmarshal json body into DecisionBody
	var decisionBody types.DecisionBody

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
	[]types.ReturnMakerRequest,
	error,
) {
	currentMakerRequest, err := FetchMakerRequestsByReqIdAndCheckerRole(reqId, checkerRole, makerTableName, req, dynaClient)
	if err != nil {
		return nil, err
	}
	if len(currentMakerRequest) == 0 || len(currentMakerRequest[0].RequestUUID) == 0 {
		return nil, errors.New(types.ErrorMakerDoesNotExist)
	}

	if decision == "approve" {
		resourceType := currentMakerRequest[0].ResourceType

		// if maker request to change user table
		if resourceType == "user" {
			var userData types.User
			if err := json.Unmarshal(currentMakerRequest[0].RequestData, &userData); err != nil {
				return nil, errors.New(types.ErrorFailedToUnmarshalRecord)
			}

			_, err = FetchUserByID(userData.User_ID, req, userTableName, dynaClient)
			if err != nil {
				return nil, errors.New(types.ErrorUserDoesNotExist)
			}

			if len(userData.User_ID) == 0 {
				return nil, errors.New(types.ErrorInvalidUserID)
			}
			// make changes to user table
			_, err := UpdateUser(userData, req, userTableName, dynaClient)
			if err != nil {
				return nil, err
			}

			// if maker request to change points table
		} else if resourceType == "points" {
			var pointsData types.UserPoint
			if err := json.Unmarshal(currentMakerRequest[0].RequestData, &pointsData); err != nil {
				return nil, errors.New(types.ErrorCouldNotMarshalItem)
			}
			_, err = FetchUserPoint(pointsData.User_ID, req, pointsTableName, dynaClient)
			if err != nil {
				return nil, errors.New(types.ErrorPointsDoesNotExist)
			}

			// make changes to points table
			_, err := UpdateUserPoint(pointsData, req, pointsTableName, dynaClient)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New(types.ErrorInvalidResourceType)
		}
		decision = "approved"
	} else if decision == "reject" {
		decision = "rejected"
	} else {
		return nil, errors.New(types.ErrorInvalidDecision)
	}

	makerRequests, err := FetchMakerRequest(reqId, makerTableName, req, dynaClient)
	if err != nil {
		return nil, err
	}
	for i, request := range makerRequests {
		request.RequestStatus = decision
		request.CheckerUUID = checkerUUID

		makerRequests[i] = request
	}

	retMakerRequest, err := utility.BatchWriteToDynamoDB(len(makerRequests), makerRequests, makerTableName, dynaClient)
	if err != nil {
		return nil, err
	}
	return retMakerRequest, nil
}

func FetchMakerRequest(requestID, tableName string, req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) ([]types.MakerRequest, error) {
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("req_id = :req_id"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":req_id": {S: aws.String(requestID)},
		},
	}

	result, err := dynaClient.Query(queryInput)

	if err != nil {
		return nil, errors.New(types.ErrorCouldNotQueryDB)
	}

	if len(result.Items) == 0 {
		return nil, errors.New(types.ErrorMakerDoesNotExist)
	}
	makerRequests := new([]types.MakerRequest)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, makerRequests)
	if err != nil {
		return nil, errors.New(types.ErrorCouldNotMarshalItem)
	}

	return *makerRequests, nil
}

func FetchMakerRequestsByReqIdAndCheckerRole(reqID, checkerRole, tableName string, req events.APIGatewayProxyRequest, dynaClient dynamodbiface.DynamoDBAPI) ([]types.MakerRequest, error) {
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
		return nil, errors.New(types.ErrorCouldNotQueryDB)
	}

	makerRequests := new([]types.MakerRequest)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, makerRequests)

	if err != nil {
		return nil, errors.New(types.ErrorCouldNotMarshalItem)
	}

	if len(*makerRequests) == 0 {
		return nil, errors.New(types.ErrorMakerDoesNotExist)
	}

	return *makerRequests, nil
}

func UpdateUser(user types.User, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*types.User, error) {
	if user.User_ID == "" {
		err := errors.New(types.ErrorInvalidUserID)
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
		return nil, errors.New(types.ErrorFailedToFetchRecordID)
	}

	if result.Item == nil {
		return nil, errors.New("user does not exist")
	}

	av, err := dynamodbattribute.MarshalMap(user)
	if err != nil {
		return nil, errors.New(types.ErrorCouldNotMarshalItem)
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = dynaClient.PutItem(input)
	if err != nil {
		return nil, errors.New(types.ErrorCouldNotDynamoPutItem)
	}

	return &user, nil
}

func FetchUserByID(id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*types.User, error) {
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
		return nil, errors.New(types.ErrorFailedToFetchRecordID)
	}

	if result.Item == nil {
		return nil, errors.New("user does not exist")
	}

	item := new(types.User)
	err = dynamodbattribute.UnmarshalMap(result.Item, item)
	if err != nil {
		return nil, errors.New(types.ErrorFailedToUnmarshalRecord)
	}

	return item, nil
}

func UpdateUserPoint(userpoint types.UserPoint, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*types.UserPoint, error) {
	// check if points id is empty
	if userpoint.Points_ID == "" {
		err := errors.New(types.ErrorInvalidPointsID)
		return nil, err
	}

	//checking if userpoint exist
	results, err := FetchUserPoint(userpoint.User_ID, req, tableName, dynaClient)
	if err != nil {
		return nil, errors.New(types.ErrorInvalidUserData)
	}

	var result = new(types.UserPoint)
	for _, v := range *results {
		if v.Points_ID == userpoint.Points_ID {
			result = &userpoint
		}
	}

	if result.Points_ID != userpoint.Points_ID {
		return nil, errors.New(types.ErrorCouldNotMarshalItem)
	}

	av, err := dynamodbattribute.MarshalMap(result)
	if err != nil {
		return nil, errors.New(types.ErrorCouldNotMarshalItem)
	}

	//updating user point in dynamo
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}
	_, err = dynaClient.PutItem(input)
	if err != nil {
		return nil, errors.New(types.ErrorCouldNotDynamoPutItem)
	}

	return result, nil
}

func FetchUserPoint(user_id string, req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI) (*[]types.UserPoint, error) {
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
		return nil, errors.New(types.ErrorFailedToFetchRecord)
	}

	if result.Items == nil {
		return nil, errors.New("user point does not exist")
	}

	item := new([]types.UserPoint)
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, item)
	if err != nil {
		return nil, errors.New(types.ErrorFailedToUnmarshalRecord)
	}

	return item, nil
}

func main() {
	lambda.Start(handler)
}
