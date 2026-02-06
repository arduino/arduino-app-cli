// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package edgeimpulse

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type EIClient struct {
	ApiUrl     url.URL
	PrjApiKey  string
	HttpClient *ClientWithResponses
}

var ErrInternalServerErr = fmt.Errorf("service unavailable")
var ErrUnauthorized = fmt.Errorf("unauthorized")

type JobLogEntry []struct {
	Created  time.Time                        `json:"created"`
	Data     string                           `json:"data"`
	LogLevel *LogStdoutResponseStdoutLogLevel `json:"logLevel,omitempty"`
}

type LastBuild *struct {
	// Created The time this build was created
	Created time.Time `json:"created"`

	// DeploymentType Deployment type of the build
	DeploymentType string                 `json:"deploymentType"`
	Engine         DeploymentTargetEngine `json:"engine"`
	ModelType      *KerasModelTypeEnum    `json:"modelType,omitempty"`

	// Version The build version, incremented after each deployment build
	Version int `json:"version"`
}

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

func (c *EIClient) DownloadAndInstallModel(ctx context.Context, projectID int, impulseID int, modelType ModelTypeParameter, engine ModelEngineParameter, deviceType DeploymentTypeParameter) (io.ReadCloser, error) {

	opt := &DownloadBuildParams{ImpulseId: &impulseID, ModelType: &modelType, Engine: &engine, Type: deviceType}

	resp, err := c.HttpClient.DownloadBuild(ctx, projectID, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to perform download model request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errorMessage(resp.StatusCode)
	}

	return resp.Body, nil
}

func (c *EIClient) GetInfoLastDeployment(ctx context.Context, projectID int, impulseID int, devicesTarget string) (LastBuild, error) {

	params := &GetLastDeploymentBuildParams{ImpulseId: &impulseID}
	resp, err := c.HttpClient.GetLastDeploymentBuildWithResponse(ctx, projectID, params)

	if err != nil {
		return nil, fmt.Errorf("failed to perform download model request: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errorMessage(resp.StatusCode())
	}

	if resp.JSON200.HasBuild && resp.JSON200.LastDeploymentTarget.Format == devicesTarget {
		return resp.JSON200.LastBuild, nil
	}
	return nil, nil
}

func (c *EIClient) GetDeployment(ctx context.Context, projectID int, impulseID int, modelType *ModelTypeParameter, engine *ModelEngineParameter, deviceType DeploymentTypeParameter) (*int, error) {

	params := &GetDeploymentParams{ModelType: modelType, Engine: engine, ImpulseId: &impulseID, Type: deviceType}
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
	} else {
		if resp.JSON200.Error != nil {
			return nil, fmt.Errorf("error fetching project info: %s", *resp.JSON200.Error)
		}
		return nil, fmt.Errorf("error fetching project info: unknown error")
	}

}

func (c *EIClient) Build(ctx context.Context, projectID int, impulseID int, modelType ModelTypeParameter, engine ModelEngineParameter, deviceType DeploymentTypeParameter) (*int, error) {

	params := &BuildOnDeviceModelJobParams{Type: deviceType, ImpulseId: &impulseID}

	km_variant := KerasModelVariantEnum(string(modelType))

	body := BuildOnDeviceModelJobJSONRequestBody{
		Engine:    engine,
		ModelType: &km_variant,
	}

	resp, err := c.HttpClient.BuildOnDeviceModelJobWithResponse(ctx, projectID, params, body)
	if err != nil {
		return nil, fmt.Errorf("failed to perform build model request: %w", err)
	}

	if resp.JSON200.Success {
		return &resp.JSON200.Id, nil
	} else {
		if resp.JSON200.Error != nil {
			return nil, fmt.Errorf("error fetching project info: %s", *resp.JSON200.Error)
		}
		return nil, fmt.Errorf("error fetching project info: unknown error")
	}

}

func (c *EIClient) GetJobStatus(ctx context.Context, projectID int, jobID int) (bool, error) {

	resp, err := c.HttpClient.GetJobStatusWithResponse(ctx, projectID, jobID)
	if err != nil {
		return false, err
	}
	if resp.StatusCode() != http.StatusOK {
		return false, errorMessage(resp.StatusCode())
	}

	if resp.JSON200.Success {
		if resp.JSON200.Job.Finished != nil && resp.JSON200.Job.FinishedSuccessful != nil {
			if *resp.JSON200.Job.FinishedSuccessful {
				return true, nil
			} else {
				logs, err := c.getJobLogs(ctx, projectID, jobID, 1, "error")
				if err != nil {
					return false, fmt.Errorf("failed to get job logs: %w", err)
				}
				if len(logs) == 0 {
					return false, fmt.Errorf("job %d failed with unknown error", jobID)
				}
				return false, fmt.Errorf("job %d failed with error: %v", jobID, logs[0].Data)
			}
		}
	} else {
		if resp.JSON200.Error != nil {
			return false, fmt.Errorf("error fetching project info: %s", *resp.JSON200.Error)
		}else{
			return false, fmt.Errorf("error fetching project info")
	     }
	  }
}

func (c *EIClient) getJobLogs(ctx context.Context, projectID, jobID int, limit int, logLevel string) (logs JobLogEntry, err error) {

	logLevelParam := GetJobsLogsParamsLogLevel(logLevel)
	resp, err := c.HttpClient.GetJobsLogsWithResponse(ctx, projectID, jobID, &GetJobsLogsParams{Limit: &limit, LogLevel: &logLevelParam})
	if err != nil {
		return nil, fmt.Errorf("failed to perform get logs request: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, errorMessage(resp.StatusCode())
	}

	return resp.JSON200.Stdout, nil
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
	} else {
		if resp.JSON200.Error != nil {
			return nil, fmt.Errorf("error fetching project info: %s", *resp.JSON200.Error)
		}
		return nil, fmt.Errorf("error fetching project info: unknown error")
	}
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
				return fmt.Errorf("Build failed for job %d in project %d", jobID, projectID)
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}

}

func errorMessage(statusCode int) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		return ErrInternalServerErr
	}
}
