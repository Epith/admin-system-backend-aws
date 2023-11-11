package types

type User struct {
	Email     string `json:"email"`
	User_ID   string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}

type CognitoUser struct {
	*User
	Password string `json:"password"`
}

type ReturnUserData struct {
	Data []User `json:"data"`
	Key  string `json:"key"`
}
