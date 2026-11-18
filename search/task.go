package search

type Task struct {
	UID        int64  `json:"uid"`
	TaskUID    int64  `json:"taskUid"`
	IndexUID   string `json:"indexUid"`
	Status     string `json:"status"`
	Type       string `json:"type"`
	EnqueuedAt string `json:"enqueuedAt"`
	Error      *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}
