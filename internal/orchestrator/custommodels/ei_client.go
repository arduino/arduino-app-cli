package custommodels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/arduino/go-paths-helper"
)

type EIClient struct {
	ApiUrl    string
	APIKey    string
	ModelRoot paths.Path
	Client    *http.Client
}

type GetDeploymentResponse struct {
	Success       bool   `json:"success"`
	HasDeployment bool   `json:"hasDeployment"`
	Error         string `json:"error"`
	Version       int    `json:"version"`
}

type BuildJobResponse struct {
	Success bool   `json:"success"`
	JobID   int    `json:"id"`
	Error   string `json:"error"`
}

type JobStatusResponse struct {
	Success bool   `json:"success"`
	Job     Job    `json:"job"`
	Error   string `json:"error"`
}

type Job struct {
	ID                  int                    `json:"id"`
	Category            string                 `json:"category"`
	CategoryKey         string                 `json:"categoryKey"`
	Key                 string                 `json:"key"`
	Created             time.Time              `json:"created"`
	JobNotificationUids []int                  `json:"jobNotificationUids"`
	Started             time.Time              `json:"started"`
	Finished            time.Time              `json:"finished"`
	FinishedSuccessful  bool                   `json:"finishedSuccessful"`
	AdditionalInfo      string                 `json:"additionalInfo"`
	ComputeTime         int                    `json:"computeTime"`
	CreatedByUser       User                   `json:"createdByUser"`
	CategoryCount       int                    `json:"categoryCount"`
	Metadata            map[string]interface{} `json:"metadata"`
}

type User struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Photo    string `json:"photo"`
}

type Project struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Created      time.Time `json:"created"`
	Owner        string    `json:"owner"`
	Tier         string    `json:"tier"`
	IsPublic     bool      `json:"isPublic"`
	Tags         []string  `json:"tags"`
	Category     string    `json:"category"`
	License      string    `json:"license"`
	LastAccessed time.Time `json:"lastAccessed"`
	LastModified time.Time `json:"lastModified"`
}

type ProjectResponse struct {
	Project Project `json:"project"`
}

func NewEIClient(apiKey string, ApiUrl string, modelRoot paths.Path) *EIClient {
	return &EIClient{APIKey: apiKey, ApiUrl: ApiUrl, ModelRoot: modelRoot, Client: http.DefaultClient}
}

func (c *EIClient) DownloadAndInstallModel(ctx context.Context, modelPath, projectID, impulseID string) error {
	queryParams := map[string]string{
		"type":      "arduino-uno-q",
		"modelType": "int8",   //TODO make it configurable
		"engine":    "tflite", //TODO make it configurable
		"impulseId": impulseID,
	}

	reader, err := c.doRequest(ctx, "GET", fmt.Sprintf("/v1/api/%s/deployment/download", projectID), queryParams, nil, nil)
	if err != nil {
		return err
	}
	defer reader.Close()

	outFile, err := os.Create(modelPath + "/model.eim")
	if err != nil {
		return fmt.Errorf("failed to create model file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, reader)
	if err != nil {
		return fmt.Errorf("failed to write model file: %w", err)
	}

	return nil
}

func (c *EIClient) GetDeployment(ctx context.Context, projectID string) (*int, error) {
	queryParams := map[string]string{
		"type":      "arduino-uno-q",
		"modelType": "int8",
		"engine":    "tflite",
	}
	var resp GetDeploymentResponse
	reader, err := c.doRequest(ctx, "GET", fmt.Sprintf("/v1/api/%s/deployment", projectID), queryParams, nil, &resp)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	if resp.Success {
		if resp.HasDeployment {
			return &resp.Version, nil
		}
		return nil, nil
	}

	return nil, fmt.Errorf("Error fetching deployment info: %s", resp.Error)
}

func (c *EIClient) Build(ctx context.Context, projectID string) (*string, error) {

	queryParams := map[string]string{
		"type": "arduino-uno-q",
	}

	body := map[string]interface{}{
		"engine":     "tflite",
		"modelType":  "int8",
		"parameters": map[string]interface{}{},
	}
	var resp BuildJobResponse
	reader, err := c.doRequest(ctx, "POST", fmt.Sprintf("/v1/api/%s/jobs/build-ondevice-model", projectID), queryParams, body, &resp)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	if resp.Success {
		jobId := fmt.Sprintf("%d", resp.JobID)
		return &jobId, nil
	}

	return nil, fmt.Errorf("Error building model: %s", resp.Error)

}

func (c *EIClient) GetJobStatus(ctx context.Context, projectID string, jobID string) (*bool, error) {

	var resp JobStatusResponse
	reader, err := c.doRequest(ctx, "GET", fmt.Sprintf("/v1/api/%s/jobs/%s/status", projectID, jobID), nil, nil, &resp)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	fmt.Println(resp)
	if resp.Success {
		return &resp.Job.FinishedSuccessful, nil
	}

	return nil, fmt.Errorf("Error fetching job status: %s", resp.Error)

}

func (c *EIClient) GetProjectInfo(ctx context.Context, projectID string) (*Project, error) {

	var resp ProjectResponse
	reader, err := c.doRequest(ctx, "GET", fmt.Sprintf("/v1/api/%s", projectID), nil, nil, &resp)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return &resp.Project, nil
}

func (c EIClient) WaitForBuildCompletion(ctx context.Context, projectID, jobID string) error {

	for {
		status, err := c.GetJobStatus(ctx, projectID, jobID)
		if err != nil {
			return err
		}
		fmt.Println("Build status:", *status)

		if *status {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}

}

// TODO if the res.body is not used, close the body directly in doRequest
func (c *EIClient) doRequest(ctx context.Context, method, path string, queryParams map[string]string, body interface{}, result interface{}) (io.ReadCloser, error) {
	// Build URL
	u, err := url.JoinPath(c.ApiUrl, path)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	// Query params
	if len(queryParams) > 0 {
		q := url.Values{}
		for k, v := range queryParams {
			q.Add(k, v)
		}
		u += "?" + q.Encode()
	}

	// Serialize body
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	// Build request
	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("x-api-key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Do request
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Handle non-2xx responses
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("non-2xx response: %d: %s", resp.StatusCode, string(b))
	}

	// Decode response
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return resp.Body, nil

}
