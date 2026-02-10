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
var ErrForbidden = fmt.Errorf("cannot access the resource with the provided credentials")

type JobLogEntry struct {
	Created  time.Time                        `json:"created"`
	Data     string                           `json:"data"`
	LogLevel *LogStdoutResponseStdoutLogLevel `json:"logLevel,omitempty"`
}

type LastBuild struct {
	Created        time.Time              `json:"created"`
	DeploymentType string                 `json:"deploymentType"`
	Engine         DeploymentTargetEngine `json:"engine"`
	ModelType      *KerasModelTypeEnum    `json:"modelType,omitempty"`
	Version        int                    `json:"version"`
}

type JobBuildInfo struct {
	JobID             int `json:"jobId"`
	DeploymentVersion int `json:"deploymentVersion"`
}
type ImpulseState struct {
	Complete   bool `json:"complete"`
	Configured bool `json:"configured"`
	Created    bool `json:"created"`
}

type ProjectImpulse struct {
	Details      Project
	ImpulseState ImpulseState
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

func (c *EIClient) DownloadHistoricDeployment(ctx context.Context, projectID int, version int) (io.ReadCloser, error) {

	resp, err := c.HttpClient.DownloadHistoricDeployment(ctx, projectID, version)
	if err != nil {
		return nil, fmt.Errorf("failed to perform download model request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode), string(b))
	}

	return resp.Body, nil
}

func (c *EIClient) GetInfoLastDeployment(ctx context.Context, projectID int, impulseID int, devicesTarget string) (*LastBuild, error) {

	params := &GetLastDeploymentBuildParams{ImpulseId: &impulseID}
	resp, err := c.HttpClient.GetLastDeploymentBuildWithResponse(ctx, projectID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to perform download model request: %w", err)
	}

	if resp.StatusCode() != http.StatusOK || !resp.JSON200.Success {
		if resp.JSON200 != nil && resp.JSON200.Error != nil {
			return nil, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), *resp.JSON200.Error)
		}
		return nil, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), string(resp.Body))
	}

	if resp.JSON200.HasBuild && resp.JSON200.LastDeploymentTarget.Format == devicesTarget {
		return (*LastBuild)(resp.JSON200.LastBuild), nil
	} else {
		return nil, nil
	}
}

func (c *EIClient) Build(ctx context.Context, projectID int, impulseID int, modelType string, engine string, deviceType DeploymentTypeParameter) (JobBuildInfo, error) {

	params := &BuildOnDeviceModelJobParams{Type: deviceType, ImpulseId: &impulseID}
	km_variant := KerasModelVariantEnum(modelType)
	body := BuildOnDeviceModelJobJSONRequestBody{
		Engine:    DeploymentTargetEngine(engine),
		ModelType: &km_variant,
	}
	resp, err := c.HttpClient.BuildOnDeviceModelJobWithResponse(ctx, projectID, params, body)
	if err != nil {
		return JobBuildInfo{}, fmt.Errorf("failed to perform build model request: %w", err)
	}

	if resp.StatusCode() != http.StatusOK || !resp.JSON200.Success {
		if resp.JSON200 != nil && resp.JSON200.Error != nil {
			return JobBuildInfo{}, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), *resp.JSON200.Error)
		}
		return JobBuildInfo{}, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), string(resp.Body))
	}

	return JobBuildInfo{JobID: resp.JSON200.Id, DeploymentVersion: resp.JSON200.DeploymentVersion}, nil
}

func (c *EIClient) isJobDone(ctx context.Context, projectID int, jobID int) (bool, error) {

	resp, err := c.HttpClient.GetJobStatusWithResponse(ctx, projectID, jobID)
	if err != nil {
		return false, err
	}

	if resp.StatusCode() != http.StatusOK || !resp.JSON200.Success {
		if resp.JSON200 != nil && resp.JSON200.Error != nil {
			return false, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), *resp.JSON200.Error)
		}
		return false, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), string(resp.Body))
	}

	if resp.JSON200.Job.Finished == nil {
		// Job not finished yet
		return false, nil
	}

	if resp.JSON200.Job.FinishedSuccessful == nil || !*resp.JSON200.Job.FinishedSuccessful {
		logs, err := c.getJobLogs(ctx, projectID, jobID, 1, "error")
		if err != nil {
			return false, fmt.Errorf("failed to get job logs: %w", err)
		}
		if len(logs) == 0 {
			return false, fmt.Errorf("job %d failed with unknown error", jobID)
		}
		return false, fmt.Errorf("job %d failed with error: %v", jobID, logs[0].Data)
	}

	return true, nil

}

func (c *EIClient) getJobLogs(ctx context.Context, projectID, jobID int, limit int, logLevel string) ([]JobLogEntry, error) {

	logLevelParam := GetJobsLogsParamsLogLevel(logLevel)
	resp, err := c.HttpClient.GetJobsLogsWithResponse(ctx, projectID, jobID, &GetJobsLogsParams{Limit: &limit, LogLevel: &logLevelParam})
	if err != nil {
		return nil, fmt.Errorf("failed to perform get logs request: %w", err)
	}

	if resp.StatusCode() != http.StatusOK || !resp.JSON200.Success {
		if resp.JSON200 != nil && resp.JSON200.Error != nil {
			return nil, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), *resp.JSON200.Error)
		}
		return nil, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), string(resp.Body))
	}

	logs := make([]JobLogEntry, 0, len(resp.JSON200.Stdout))
	for _, log := range resp.JSON200.Stdout {
		logs = append(logs, JobLogEntry(log))
	}

	return logs, nil
}

func (c *EIClient) GetProjectInfo(ctx context.Context, projectID int, impulseID int) (ProjectImpulse, error) {

	resp, err := c.HttpClient.GetProjectInfoWithResponse(ctx, projectID, &GetProjectInfoParams{ImpulseId: &impulseID})
	if err != nil {
		return ProjectImpulse{}, fmt.Errorf("failed to perform get project info request: %w", err)
	}

	if resp.StatusCode() != http.StatusOK || !resp.JSON200.Success {
		if resp.JSON200 != nil && resp.JSON200.Error != nil {
			return ProjectImpulse{}, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), *resp.JSON200.Error)
		}
		return ProjectImpulse{}, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), string(resp.Body))
	}

	return ProjectImpulse{
		Details:      resp.JSON200.Project,
		ImpulseState: resp.JSON200.Impulse,
	}, nil
}

func (c *EIClient) GetDeploymentHistory(ctx context.Context, projectID int, impulseID int, limit int) ([]DeploymentHistory, error) {

	params := &ListDeploymentHistoryParams{ImpulseId: &impulseID, Limit: &limit}
	resp, err := c.HttpClient.ListDeploymentHistoryWithResponse(ctx, projectID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to perform get deployment history request: %w", err)
	}

	if resp.StatusCode() != http.StatusOK || !resp.JSON200.Success {
		if resp.JSON200 != nil && resp.JSON200.Error != nil {
			return nil, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), *resp.JSON200.Error)
		}
		return nil, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), string(resp.Body))
	}

	return resp.JSON200.Deployments, nil
}
func (c *EIClient) GetImpulseInfo(ctx context.Context, projectID int, impulseID int) (*Impulse, error) {
	params := &GetImpulseParams{ImpulseId: &impulseID}
	resp, err := c.HttpClient.GetImpulseWithResponse(ctx, projectID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to perform get impulse request: %w", err)
	}

	if resp.StatusCode() != http.StatusOK || !resp.JSON200.Success {
		if resp.JSON200 != nil && resp.JSON200.Error != nil {
			return nil, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), *resp.JSON200.Error)
		}
		return nil, fmt.Errorf("%w: %s", errorMessage(resp.StatusCode()), string(resp.Body))
	}

	return resp.JSON200.Impulse, nil
}

func (c EIClient) WaitForBuildCompletion(ctx context.Context, projectID, jobID int) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	for {
		status, err := c.isJobDone(ctx, projectID, jobID)
		if err != nil {
			return err
		}

		if status {
			return nil
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
	case http.StatusForbidden:
		return ErrForbidden
	default:
		return ErrInternalServerErr
	}
}
