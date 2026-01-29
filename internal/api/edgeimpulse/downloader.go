package edgeimpulse

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go.bug.st/f"
)

type ProjectDownloader struct {
	client    *ClientWithResponses
	ProjectID int

	deploymentType DeploymentTypeParameter

	// OnBuildProgress func(string)
}

func NewProjectDownloader(
	client *ClientWithResponses,
	projectID int,
) *ProjectDownloader {
	return &ProjectDownloader{
		ProjectID:      projectID,
		client:         client,
		deploymentType: "runner-linux-aarch64", // TODO: make dynamic based on board ?
	}
}

func (r *ProjectDownloader) GetProjectInfo(ctx context.Context, impulseID *OptionalImpulseIdParameter) (*Project, error) {
	resp, err := r.client.GetProjectInfoWithResponse(ctx, r.ProjectID, &GetProjectInfoParams{ImpulseId: impulseID})
	if err != nil {
		return nil, fmt.Errorf("failed to perform get project info request: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get project info: status code %d", resp.StatusCode())
	}
	if resp.JSON200.Success {
		return &resp.JSON200.Project, nil
	}

	return nil, fmt.Errorf("error fetching project info: %s", *resp.JSON200.Error)

}

func (r *ProjectDownloader) DownloadDeployment(ctx context.Context, impulseID *OptionalImpulseIdParameter, modelType ModelTypeParameter) (io.ReadCloser, error) {
	resp, err := r.client.GetDeploymentWithResponse(ctx, r.ProjectID,
		&GetDeploymentParams{
			Type:      r.deploymentType,
			ModelType: &modelType,
			ImpulseId: impulseID,
			// use preferred engine
			Engine: nil,
		})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("no deployment available")
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to get deployment info: status code %d", resp.StatusCode())
	}

	if !resp.JSON200.HasDeployment {
		if err := r.buildModel(ctx, impulseID, modelType); err != nil {
			return nil, err
		}
	}

	respDownload, err := r.client.DownloadBuildWithResponse(ctx, r.ProjectID,
		&DownloadBuildParams{
			Type:      r.deploymentType,
			ModelType: &modelType,
			ImpulseId: impulseID,
			// use preferred engine
			Engine: nil,
		})
	if err != nil {
		return nil, err
	}
	if respDownload.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to download deployment: status code %d", respDownload.StatusCode())
	}

	return io.NopCloser(bytes.NewReader(respDownload.Body)), nil
}

func (r *ProjectDownloader) buildModel(ctx context.Context, impulseID *OptionalImpulseIdParameter, modelType ModelTypeParameter) error {
	// list all deploy targets
	resp, err := r.client.ListDeploymentTargetsForProjectWithResponse(ctx, r.ProjectID, &ListDeploymentTargetsForProjectParams{
		ImpulseId: impulseID,
	})
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("failed to get deployment targets: status code %d", resp.StatusCode())
	}
	dt := resp.JSON200
	if !dt.Success {
		return fmt.Errorf("failed to get deployment targets: %s", *dt.Error)
	}

	var target *ProjectDeploymentTarget
	for _, t := range dt.Targets {
		if t.Format == r.deploymentType {
			target = &t
			break
		}
	}
	if target == nil {
		return errors.New("Failed to find deployment type " + r.deploymentType + ", types found: " + strings.Join(f.Map(dt.Targets, func(x ProjectDeploymentTarget) string { return x.Format }), ", "))
	}

	engine := target.PreferredEngine

	res, err := r.client.BuildOnDeviceModelJobWithResponse(ctx,
		r.ProjectID,
		&BuildOnDeviceModelJobParams{
			Type:      r.deploymentType,
			ImpulseId: impulseID,
		},
		BuildOnDeviceModelRequest{
			Engine:    engine,
			ModelType: &modelType,
		},
	)
	if err != nil {
		return err
	}
	if res.StatusCode() != 200 {
		slog.Error("failed to start build model job", slog.Int("status_code", res.StatusCode()), slog.Any("response", res.Body))
		return fmt.Errorf("failed to start build model job: status code %d", res.StatusCode())
	}
	if !res.JSON200.Success {
		return fmt.Errorf("failed to start build model job: %s", *res.JSON200.Error)
	}

	if err := r.runJobUntileComletion(ctx, res.JSON200.Id); err != nil {
		return err
	}

	return nil
}

// runJobUntileComletion polls the job status until completion, streaming logs via OnBuildProgress.
func (r *ProjectDownloader) runJobUntileComletion(ctx context.Context, jobID JobIdParameter) error {
	var respJob *GetJobStatusHTTPResponse
	for {
		var err error
		respJob, err = r.client.GetJobStatusWithResponse(ctx,
			r.ProjectID,
			jobID,
		)
		if err != nil {
			return err
		}
		if respJob.StatusCode() != 200 {
			return fmt.Errorf("failed to get job status: status code %d", respJob.StatusCode())
		}
		if !respJob.JSON200.Success {
			return fmt.Errorf("failed to get job status: %s", *respJob.JSON200.Error)
		}

		if respJob.JSON200.Job.FinishedSuccessful != nil && *respJob.JSON200.Job.FinishedSuccessful {
			break
		}

		// wait and poll again
		time.Sleep(5 * time.Second)
	}

	if respJob.JSON200.Job.FinishedSuccessful == nil || !*respJob.JSON200.Job.FinishedSuccessful {
		return errors.New("model build job failed")
	}

	return nil
}
