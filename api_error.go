package nestapi

import "strings"

/*
Error is the interface that includes the specific reason for the error
*/
type Error interface {
	error
	Reason() string
}

/*
APIError includes the available Nest API error information
See: https://developer.nest.com/documentation/cloud/error-messages
*/

type APIError struct {
	OldError string `json:"error"`
	Type     string `json:"type"`
	Message  string `json:"message"`
	Instance string `json:"instance"`

	// Set as an interface since it comes in as diffrent things and we
	// can't know ahead of time what they will be. We don't really use
	// this anyway, but here for completeness.
	Details interface{} `json:"details"`
}

/*
Error satisfies the error interface
*/
func (n *APIError) Error() string {
	return (n.Reason() + "||" + n.HumanMessage())
}

/*
Reason satisfies the nestapi Error interface
*/
func (n *APIError) Reason() string {
	return strings.Split(n.Type, "#")[1]
}

/*
HumanMessage returns better error messages for older errors
*/
func (n *APIError) HumanMessage() string {
	switch n.Reason() {
	case "blocked":
		return "The Nest API has blocked further requests. Please try again later."
	case "not-found":
		return "The information requested was not found."
	case "auth-error":
		return "Your are not authorized to view the Nest account."
	case "forbidden", "service-unavailable":
		return "There is an issue with the Nest service. Please try again later."
	case "unknown":
		return "An unknown error has occurred on the Nest service."
	}
	return n.Message
}
