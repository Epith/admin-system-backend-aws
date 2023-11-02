package main

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/google/uuid"
)

type User struct {
	Email     string `json:"email"`
	User_ID   string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

var (
	ErrorFailedToUnmarshalRecord  = "failed to unmarshal record"
	ErrorFailedToFetchRecord      = "failed to fetch record"
	ErrorFailedToFetchRecordID    = "failed to fetch record by uuid"
	ErrorFailedToFetchRecordEmail = "failed to fetch record by email"
	ErrorInvalidUserData          = "invalid user data"
	ErrorInvalidEmail             = "invalid email"
	ErrorInvalidFirstName         = "invalid first name"
	ErrorInvalidLastName          = "invalid last name"
	ErrorInvalidUUID              = "invalid UUID"
	ErrorCouldNotMarshalItem      = "could not marshal item"
	ErrorCouldNotDeleteItem       = "could not delete item"
	ErrorCouldNotDynamoPutItem    = "could not dynamo put item"
	ErrorUserAlreadyExists        = "user.User already exists"
	ErrorUserDoesNotExist         = "user.User does not exist"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
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
	res, err := CreateUser(request, USER_TABLE, dynaClient, cognitoClient)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
		}, err
	}
	stringBody, _ := json.Marshal(res)
	return events.APIGatewayProxyResponse{
		Body:       string(stringBody),
		StatusCode: 200,
	}, nil
}

func CreateUser(req events.APIGatewayProxyRequest, tableName string, dynaClient dynamodbiface.DynamoDBAPI, cognitoClient CognitoIdentityProvider) (
	*User,
	error,
) {
	var user User

	if err := json.Unmarshal([]byte(req.Body), &user); err != nil {
		return nil, errors.New(ErrorInvalidUserData)
	}
	if !IsEmailValid(user.Email) {
		return nil, errors.New(ErrorInvalidEmail)
	}
	if len(user.FirstName) == 0 {
		return nil, errors.New(ErrorInvalidFirstName)
	}
	if len(user.LastName) == 0 {
		return nil, errors.New(ErrorInvalidLastName)
	}
	user.User_ID = uuid.NewString()

	// currentUser, _ := FetchUserByEmail(user.Email, tableName, dynaClient)
	// if currentUser != nil && len(currentUser.Email) != 0 {
	// 	return nil, errors.New(ErrorUserAlreadyExists)
	// }
	av, err := dynamodbattribute.MarshalMap(user)

	if err != nil {
		return nil, errors.New(ErrorCouldNotMarshalItem)
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}
	_, err = dynaClient.PutItem(input)
	if err != nil {
		return nil, errors.New(ErrorCouldNotDynamoPutItem)
	}

	cognitoInput := &cognitoidentityprovider.AdminCreateUserInput{
		DesiredDeliveryMediums: []*string{
			aws.String("EMAIL"),
		},
		MessageAction: aws.String("SUPPRESS"),
		UserAttributes: []*cognitoidentityprovider.AttributeType{
			{
				Name:  aws.String("name"),
				Value: aws.String(user.FirstName + user.LastName),
			},
			{
				Name:  aws.String("email"),
				Value: aws.String(user.Email),
			},
			{
				Name:  aws.String("custom:role"),
				Value: aws.String(user.Role),
			},
		},
		UserPoolId: aws.String(""),
		Username:   aws.String(user.User_ID),
	}

	_, cognitoErr := cognitoClient.AdminCreateUser(cognitoInput)
	if cognitoErr != nil {
		return nil, errors.New(cognitoidentityprovider.ErrCodeCodeDeliveryFailureException)
		// if aerr, ok := err.(awserr.Error); ok {
		// 	switch aerr.Code() {
		// 	case cognitoidentityprovider.ErrCodeResourceNotFoundException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeResourceNotFoundException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeInvalidParameterException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeInvalidParameterException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeUserNotFoundException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeUserNotFoundException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeUsernameExistsException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeUsernameExistsException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeInvalidPasswordException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeInvalidPasswordException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeCodeDeliveryFailureException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeCodeDeliveryFailureException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeUnexpectedLambdaException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeUnexpectedLambdaException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeUserLambdaValidationException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeUserLambdaValidationException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeInvalidLambdaResponseException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeInvalidLambdaResponseException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodePreconditionNotMetException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodePreconditionNotMetException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeInvalidSmsRoleAccessPolicyException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeInvalidSmsRoleAccessPolicyException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeInvalidSmsRoleTrustRelationshipException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeInvalidSmsRoleTrustRelationshipException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeTooManyRequestsException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeTooManyRequestsException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeNotAuthorizedException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeNotAuthorizedException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeUnsupportedUserStateException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeUnsupportedUserStateException, aerr.Error())
		// 	case cognitoidentityprovider.ErrCodeInternalErrorException:
		// 		fmt.Println(cognitoidentityprovider.ErrCodeInternalErrorException, aerr.Error())
		// 	default:
		// 		fmt.Println(aerr.Error())
		// 	}
		// } else {
		// 	// Print the error, cast err to awserr.Error to get the Code and
		// 	// Message from an error.
		// 	fmt.Println(err.Error())
		// }
	}
	return &user, nil
}

func IsEmailValid(email string) bool {
	var rxEmail = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]{1,64}@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

	if len(email) < 3 || len(email) > 254 || !rxEmail.MatchString(email) {
		return false
	}

	return true
}

func main() {
	lambda.Start(handler)
}
