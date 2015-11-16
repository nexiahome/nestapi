package nestapi

// Auth sets the custom NestAPI token used to authenticate to NestAPI
func (n *NestAPI) Auth(token string) {
	n.params.Set(authParam, token)
}

// Unauth removes the current token being used to authenticate to NestAPI
func (n NestAPI) Unauth() {
	n.params.Del(authParam)
}
