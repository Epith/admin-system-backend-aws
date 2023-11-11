package types

type Log struct {
	Log_ID      string `json:"log_id"`
	IP          string `json:"ip"`
	Description string `json:"description"`
	UserAgent   string `json:"user_agent"`
	Timestamp   int64  `json:"timestamp"`
	TTL         int64  `json:"ttl"`
}

type ReturnLogData struct {
	Data []Log  `json:"data"`
	Key  string `json:"key"`
}
