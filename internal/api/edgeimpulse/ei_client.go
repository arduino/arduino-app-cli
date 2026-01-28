package edgeimpulse

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/arduino/go-paths-helper"
)

type EIClient struct {
	ApiUrl     url.URL
	PrjApiKey  string
	HttpClient *ClientWithResponses
}

var InternalServerErr = fmt.Errorf("service unavailable")
var UnauthorizedErr = fmt.Errorf("unauthorized")

func NewEIClient(prjApiKey string, apiURL url.URL) (*EIClient, error) {

	ClientOptions := []ClientOption{
		WithBaseURL(apiURL.String()),
		WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Add("x-api-key", prjApiKey)
			req.Header.Set("Content-Type", "application/json")
			return nil
		}),
	}
	httpClient, err := NewClientWithResponses(apiURL.String(), ClientOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create EI OpenClient: %v", err)
	}

	return &EIClient{PrjApiKey: prjApiKey, ApiUrl: apiURL, HttpClient: httpClient}, nil
}

func (c *EIClient) DownloadAndInstallModel(ctx context.Context, modelPath *paths.Path, projectID int, impulseID int, modelType ModelTypeParameter, engine ModelEngineParameter, deviceType DeploymentTypeParameter) (io.ReadCloser, error) {

	opt := &DownloadBuildParams{ImpulseId: &impulseID, ModelType: &modelType, Engine: &engine, Type: deviceType}

	resp, err := c.HttpClient.DownloadBuildWithResponse(ctx, projectID, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to perform download model request: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errorMessage(resp.StatusCode())
	}

	return io.NopCloser(bytes.NewReader(resp.Body)), nil
}

func (c *EIClient) GetDeployment(ctx context.Context, projectID int, modelType ModelTypeParameter, engine ModelEngineParameter, deviceType DeploymentTypeParameter) (*int, error) {

	params := &GetDeploymentParams{ModelType: &modelType, Engine: &engine, Type: deviceType}
	resp, err := c.HttpClient.GetDeploymentWithResponse(ctx, projectID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to perform get deployment request: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, errorMessage(resp.StatusCode())
	}

	if resp.JSON200.Success {
		if resp.JSON200.HasDeployment {
			return resp.JSON200.Version, nil
		}
		return nil, nil
	}

	return nil, fmt.Errorf("error fetching deployment info: %s", *resp.JSON200.Error)
}

func (c *EIClient) Build(ctx context.Context, projectID int, modelType ModelTypeParameter, engine ModelEngineParameter, deviceType DeploymentTypeParameter) (*int, error) {

	params := &BuildOnDeviceModelJobParams{Type: deviceType}

	body := BuildOnDeviceModelJobJSONRequestBody{
		Engine:    engine,
		ModelType: &modelType,
	}

	resp, err := c.HttpClient.BuildOnDeviceModelJobWithResponse(ctx, projectID, params, body)
	if err != nil {
		return nil, fmt.Errorf("failed to perform build model request: %w", err)
	}

	if resp.JSON200.Success {
		return &resp.JSON200.Id, nil
	}

	return nil, fmt.Errorf("error building model: %s", *resp.JSON200.Error)

}

func (c *EIClient) GetJobStatus(ctx context.Context, projectID int, jobID int) (*bool, error) {

	resp, err := c.HttpClient.GetJobStatusWithResponse(ctx, projectID, jobID)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, errorMessage(resp.StatusCode())
	}

	if resp.JSON200.Success {
		if resp.JSON200.Job.FinishedSuccessful != nil && resp.JSON200.Job.Finished != nil {
			return resp.JSON200.Job.FinishedSuccessful, nil
		}
		return nil, nil
	}

	return nil, fmt.Errorf("error fetching job status: %s", *resp.JSON200.Error)

}

func (c *EIClient) GetProjectInfo(ctx context.Context, projectID int, impulseID int) (*Project, error) {

	resp, err := c.HttpClient.GetProjectInfoWithResponse(ctx, projectID, &GetProjectInfoParams{ImpulseId: &impulseID})
	if err != nil {
		return nil, fmt.Errorf("failed to perform get project info request: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, errorMessage(resp.StatusCode())
	}
	if resp.JSON200.Success {
		return &resp.JSON200.Project, nil
	}

	return nil, fmt.Errorf("error fetching project info: %s", *resp.JSON200.Error)

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
				return fmt.Errorf("reating deployment failed for job %d in project %d", jobID, projectID)
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}

}

func errorMessage(statusCode int) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return UnauthorizedErr
	default:
		return InternalServerErr
	}
}
