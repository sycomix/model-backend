package util

var Tasks = map[string]int{
	"TASK_CLASSIFICATION": 1,
	"TASK_DETECTION":      2,
}

var Visibility = map[string]string{
	"public":  "VISIBILITY_PUBLIC",
	"private": "VISIBILITY_PRIVATE",
}

const (
	_  = iota
	KB = 1 << (10 * iota)
	MB
	GB
	TB
)

const MaxBatchSize int = 32
const MaxImageSizeBytes int = 4 * MB

const MODEL_NAME_REGEX = "^[A-Za-z0-9][a-zA-Z0-9_.-]*$"

const USER_ID = "2a06c2f7-8da9-4046-91ea-240f88a5d729"
const TYPE_USER = "user"
