package nestapi

import "encoding/json"

// Set the value of the NestAPI reference
func (n *NestAPI) Set(v interface{}) error {
	bytes, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = n.doRequest("PUT", bytes)
	return err
}
