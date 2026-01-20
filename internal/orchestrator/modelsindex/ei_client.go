package modelsindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type EIClient struct {
	ApiUrl     string
	APIKey     string
	ApiVersion string
	HttpClient *ClientWithResponses
}

func NewEIClient(apiKey string, ApiUrl string, ApiVersion string) *EIClient {

	ClientOptions := []ClientOption{
		WithBaseURL(ApiUrl + ApiVersion),
		WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			fmt.Println("Request URL:", req.URL.String())
			req.Header.Add("x-api-key", apiKey)
			req.Header.Set("Content-Type", "application/json")
			return nil
		}),
	}
	httpClient, err := NewClientWithResponses(ApiUrl, ClientOptions...)
	if err != nil {
		panic(fmt.Sprintf("failed to create EI OpenClient: %v", err))
	}

	return &EIClient{APIKey: apiKey, ApiUrl: ApiUrl, ApiVersion: ApiVersion, HttpClient: httpClient}
}

func (c *EIClient) DownloadAndInstallModel(ctx context.Context, modelPath string, projectID int, impulseID int, modelType ModelTypeParameter, engine ModelEngineParameter) error {

	opt := &DownloadBuildParams{ImpulseId: &impulseID, ModelType: &modelType, Engine: &engine, Type: "arduino-uno-q"}

	response, err := c.HttpClient.DownloadBuildWithResponse(ctx, projectID, opt)
	if err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}

	if response.StatusCode() != http.StatusOK {
		return fmt.Errorf("failed to download model, status code: %d", response.StatusCode())
	}

	modelFolder := fmt.Sprintf("ei-model-%d-%d", projectID, impulseID)

	filepath := filepath.Join(modelPath+"/", modelFolder)

	if err := os.Mkdir(filepath, 0o755); err != nil {
		log.Fatalf("failed to create directory %s: %v", filepath, err)
	}

	err = os.WriteFile(filepath+"/model.eim", []byte(response.Status()), 0755)
	if err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

func (c *EIClient) GetDeployment(ctx context.Context, projectID int, modelType ModelTypeParameter, engine ModelEngineParameter) (*int, error) {

	params := &GetDeploymentParams{ModelType: &modelType, Engine: &engine, Type: "arduino-uno-q"}
	resp, err := c.HttpClient.GetDeploymentWithResponse(ctx, projectID, params)
	if err != nil {
		return nil, err
	}

	if resp.JSON200.Success {
		if resp.JSON200.HasDeployment {
			return resp.JSON200.Version, nil
		}
		return nil, nil
	}

	return nil, fmt.Errorf("Error fetching deployment info: %s", *resp.JSON200.Error)
}

func (c *EIClient) Build(ctx context.Context, projectID int, modelType ModelTypeParameter, engine ModelEngineParameter) (*int, error) {

	params := &BuildOnDeviceModelJobParams{Type: "arduino-uno-q"}

	//TODO is the map parameters needed?
	body := BuildOnDeviceModelJobJSONRequestBody{
		Engine:    engine,
		ModelType: &modelType,
	}

	resp, err := c.HttpClient.BuildOnDeviceModelJobWithResponse(ctx, projectID, params, body)
	if err != nil {
		return nil, err
	}

	if resp.JSON200.Success {

		return &resp.JSON200.Id, nil
	}

	return nil, fmt.Errorf("Error building model: %s", *resp.JSON200.Error)

}

func (c *EIClient) GetJobStatus(ctx context.Context, projectID int, jobID int) (*bool, error) {

	resp, err := c.HttpClient.GetJobStatusWithResponse(ctx, projectID, jobID)
	if err != nil {
		return nil, err
	}

	if resp.JSON200.Success {
		if resp.JSON200.Job.FinishedSuccessful != nil && resp.JSON200.Job.Finished != nil {
			return resp.JSON200.Job.FinishedSuccessful, nil
		}
		return nil, nil
	}

	return nil, fmt.Errorf("Error fetching job status: %s", *resp.JSON200.Error)

}

func (c *EIClient) GetProjectInfo(ctx context.Context, projectID int, impulseID int) (*Project, error) {

	resp, err := c.HttpClient.GetProjectInfoWithResponse(ctx, projectID, &GetProjectInfoParams{ImpulseId: &impulseID})
	if err != nil {
		return nil, err
	}
	if resp.JSON200.Success {
		return &resp.JSON200.Project, nil
	}

	return nil, fmt.Errorf("Error fetching project info: %s", *resp.JSON200.Error)

}

func (c EIClient) WaitForBuildCompletion(ctx context.Context, projectID, jobID int) error {

	for {
		status, err := c.GetJobStatus(ctx, projectID, jobID)
		if err != nil {
			return err
		}

		if status != nil {
			if *status {
				return nil
			} else {
				return fmt.Errorf("Creating deployment failed for job %d in project %d", jobID, projectID)
			}
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
	resp, err := http.DefaultClient.Do(req)
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
