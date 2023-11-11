package types

type UserPoint struct {
	User_ID   string `json:"user_id"`
	Points_ID string `json:"points_id"`
	Points    int    `json:"points"`
}

type ReturnUserPointData struct {
	Data     []UserPoint `json:"data"`
	KeyUser  string      `json:"key_user"`
	KeyPoint string      `json:"key_point"`
}
