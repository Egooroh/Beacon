package domain

// AlertType describes what triggered a notification.
type AlertType string

const (
	AlertNewIssue   AlertType = "new_issue"  // first occurrence of a fingerprint
	AlertRegression AlertType = "regression" // a resolved issue got a new event
	AlertSpike      AlertType = "spike"      // event rate exceeded threshold
)

// Alert carries everything needed to render a notification.
type Alert struct {
	Type    AlertType
	Issue   *Issue
	Project *Project
}
