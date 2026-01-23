package edgeimpulse

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/arduino/go-paths-helper"
)

type EIClient struct {
	ApiUrl     string
	UserToken  string
	ApiVersion string
	HttpClient *ClientWithResponses
}

func NewEIClient(userToken string, apiURL string, apiVersion string) *EIClient {

	ClientOptions := []ClientOption{
		WithBaseURL(apiURL + apiVersion),
		WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			// fmt.Println("Request URL:", req.URL.String())
			req.Header.Add("x-jwt-token", userToken)
			req.Header.Set("Content-Type", "application/json")
			return nil
		}),
	}
	httpClient, err := NewClientWithResponses(apiURL, ClientOptions...)
	if err != nil {
		panic(fmt.Sprintf("failed to create EI OpenClient: %v", err))
	}

	return &EIClient{UserToken: userToken, ApiUrl: apiURL, ApiVersion: apiVersion, HttpClient: httpClient}
}

func (c *EIClient) DownloadAndInstallModel(ctx context.Context, modelPath *paths.Path, projectID int, impulseID int, modelType ModelTypeParameter, engine ModelEngineParameter) error {

	//TODO arduino-uno-q should be parameterized
	opt := &DownloadBuildParams{ImpulseId: &impulseID, ModelType: &modelType, Engine: &engine, Type: "arduino-uno-q"}

	response, err := c.HttpClient.DownloadBuildWithResponse(ctx, projectID, opt)
	if err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}

	if response.StatusCode() != http.StatusOK {
		return fmt.Errorf("failed to download model, status code: %d", response.StatusCode())
	}

	modelFile := modelPath.Join("model.eim").String()
	err = os.WriteFile(modelFile, response.Body, 0600)
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

	return nil, fmt.Errorf("error fetching deployment info: %s", *resp.JSON200.Error)
}

func (c *EIClient) Build(ctx context.Context, projectID int, modelType ModelTypeParameter, engine ModelEngineParameter) (*int, error) {

	params := &BuildOnDeviceModelJobParams{Type: "arduino-uno-q"}

	// TODO is the map parameters needed?
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

	return nil, fmt.Errorf("error building model: %s", *resp.JSON200.Error)

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

	return nil, fmt.Errorf("error fetching job status: %s", *resp.JSON200.Error)

}

func (c *EIClient) GetProjectInfo(ctx context.Context, projectID int, impulseID int) (*Project, error) {

	resp, err := c.HttpClient.GetProjectInfoWithResponse(ctx, projectID, &GetProjectInfoParams{ImpulseId: &impulseID})
	if err != nil {
		return nil, err
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
