// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package models

import (
	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

type AIModelsListResult struct {
	Models []AIModelItem `json:"models"`
}

type AIModelItem struct {
	// Every id the API reports is encoded, and no other form comes back.
	ID           string                  `json:"id" example:"bGxhbWFjcHA6Z2VtbWEtMy0xYi1pdC1RNF8w"`
	IDDecoded    string                  `json:"id_decoded" example:"llamacpp:gemma-3-1b-it-Q4_0"`
	Name         string                  `json:"name"`
	Handler      string                  `json:"handler" description:"handler that manages the model, as the listing reports it (hf-handler, ei-handler, ai-hub-handler)"`
	Description  string                  `json:"description"`
	Runner       string                  `json:"runner"`
	Bricks       []string                `json:"brick_ids"`
	Metadata     map[string]string       `json:"metadata,omitempty"`
	Preinstalled bool                    `json:"preinstalled" description:"ships with the board image: nothing to download, cannot be deleted"`
	Origin       modelsindex.ModelOrigin `json:"origin"`
	SizeBytes    *uint64                 `json:"size_bytes,omitempty" description:"size in bytes, converted from MiB rounded to two decimals (approximate to ~5 KiB); on disk when installed, declared otherwise; omitted when unknown"`
	Status       modelsindex.ModelStatus `json:"status"`
}

// NewAIModelItem maps an index model onto the API shape, and is the only place the id is
// encoded. Size is omitted when unknown rather than reported as zero.
func NewAIModelItem(model modelsindex.AIModel) AIModelItem {
	var size *uint64
	if model.SizeBytes > 0 {
		size = &model.SizeBytes
	}
	return AIModelItem{
		ID:           EncodeModelID(model.ID),
		IDDecoded:    model.ID,
		Name:         model.Name,
		Handler:      model.Handler,
		Description:  model.Description,
		Runner:       model.Runner,
		Bricks:       f.Map(model.Bricks, func(b modelsindex.BrickConfig) string { return b.ID }),
		Metadata:     model.Metadata,
		Preinstalled: model.Preinstalled,
		Origin:       model.Origin,
		SizeBytes:    size,
		Status:       model.Status,
	}
}
