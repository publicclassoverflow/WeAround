package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
)

type Prediction struct {
	Prediction int       `json:"prediction"`
	Key        string    `json:"key"`
	Scores     []float64 `json:"scores"`
}

type MlResponse struct {
	Predictions []Prediction `json:"predictions"`
}

type ImageBytes struct {
	B64 []byte `json:"b64"`
}
type Instance struct {
	ImageBytes ImageBytes `json:"image_bytes"`
	Key        string     `json:"key"`
}

type MlRequest struct {
	Instances []Instance `json:"instances"`
}

var (
	// Replace the project name and model name with your configuration
	project = "wearound"
	// You can use your own model or the pretrained model provided by me
	model = "Pretrained_Model"
	url   = "https://ml.googleapis.com/v1/projects/" + project + "/models/" + model + ":predict"
	scope = "https://www.googleapis.com/auth/cloud-platform"
)

// Annotate an image file based on the ML model, return score and error if exist any
func annotate(r io.Reader) (float64, error) {
	// Read the image data
	ctx := context.Background()
	buf, _ := ioutil.ReadAll(r)

	// Get the token
	ts, err := google.DefaultTokenSource(ctx, scope)
	if err != nil {
		fmt.Printf("failed to create token %v\n", err)
		return 0.0, err
	}
	tt, _ := ts.Token()

	// Construct an ML request
	request := &MlRequest{
		Instances: []Instance{
			{
				ImageBytes: ImageBytes{
					B64: buf,
				},
				// The key does not matter to the client
				// It is only for tracking by Google
				Key: "1",
			},
		},
	}

	// Transform into a JSON request
	body, _ := json.Marshal(request)
	// Construct an http request.
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+tt.AccessToken)

	fmt.Printf("Sending request to ml engine for prediction %s with token as %s\n", url, tt.AccessToken)

	// Send the request to Google
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		fmt.Printf("failed to send ml request %v\n", err)
		return 0.0, err
	}

	// If everything goes on well, get the response body returning from the ML engine
	var resp MlResponse
	body, _ = ioutil.ReadAll(res.Body)

	// Double check if the response is empty
	// Sometimes Google does not return an error,
	// instead, just an empty response while usually it's due to something wrong with auth
	if len(body) == 0 {
		fmt.Println("empty google response")
		return 0.0, errors.New("empty google response")
	}
	// Try to decode the response
	if err := json.Unmarshal(body, &resp); err != nil {
		fmt.Printf("failed to parse response %v\n", err)
		return 0.0, err
	}
	// Check if the response is empty
	// If it is not, Google returns a different format. Check the raw message.
	// Sometimes it's due to the image format. Google only accepts jpeg format.
	if len(resp.Predictions) == 0 {
		fmt.Printf("failed to parse response %s\n", string(body))
		return 0.0, errors.Errorf("cannot parse response %s\n", string(body))
	}
	// Update index based on your ML model.
	results := resp.Predictions[0]
	fmt.Printf("Received a prediction result %f\n", results.Scores[0])
	return results.Scores[0], nil
}
