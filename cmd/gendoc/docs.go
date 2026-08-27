// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"log/slog"
	"net/http"
	"path"
	"reflect"
	"strings"

	"github.com/swaggest/jsonschema-go"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi3"
	"go.bug.st/f"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/arduino/arduino-app-cli/internal/api/handlers"
	"github.com/arduino/arduino-app-cli/internal/api/models"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/appid"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/update"
)

type Tag string

const (
	ApplicationTag Tag = "Application"
	BrickTag       Tag = "Brick"
	AIModelsTag    Tag = "AIModels"
	SystemTag      Tag = "System"
	Property       Tag = "Property"
	LibrariesTag   Tag = "Libraries"
)

var validTags = []Tag{ApplicationTag, BrickTag, AIModelsTag, SystemTag, LibrariesTag}

type Generator struct {
	reflector *openapi3.Reflector
}

func NewOpenApiGenerator(version string) *Generator {
	reflector := openapi3.NewReflector()
	reflector.Spec.Info.WithTitle("Arduino-App-Cli").WithVersion(version)
	reflector.Spec.Servers = append(reflector.Spec.Servers, openapi3.Server{
		URL:         "http://localhost:8800",
		Description: new("local server"),
	})

	reflector.Spec.Components = &openapi3.Components{}
	reflector.Spec.Components.Schemas = &openapi3.ComponentsSchemas{}
	reflector.Spec.Components.Schemas.WithMapOfSchemaOrRefValuesItem(
		"Status",
		openapi3.SchemaOrRef{
			Schema: &openapi3.Schema{
				UniqueItems: new(true),
				Enum:        f.Map(orchestrator.Status("").AllowedStatuses(), func(v orchestrator.Status) any { return v }),
				Type:        new(openapi3.SchemaTypeString),
				Description: new("Application status"),
				ReflectType: reflect.TypeOf(orchestrator.Status("")),
			},
		},
	)
	reflector.Spec.Components.Schemas.WithMapOfSchemaOrRefValuesItem(
		"PackageType",
		openapi3.SchemaOrRef{
			Schema: &openapi3.Schema{
				UniqueItems: new(true),
				Enum:        f.Map(update.PackageType("").AllowedStatuses(), func(v update.PackageType) any { return v }),
				Type:        new(openapi3.SchemaTypeString),
				Description: new("Package type"),
				ReflectType: reflect.TypeOf(update.PackageType("")),
			},
		},
	)
	reflector.Spec.Components.Schemas.WithMapOfSchemaOrRefValuesItem(
		"ModelStatus",
		openapi3.SchemaOrRef{
			Schema: &openapi3.Schema{
				UniqueItems: new(true),
				Enum:        f.Map(modelsindex.ModelStatus("").AllowedStatuses(), func(v modelsindex.ModelStatus) any { return v }),
				Type:        new(openapi3.SchemaTypeString),
				Description: new("Model status"),
				ReflectType: reflect.TypeOf(modelsindex.ModelStatus("")),
			},
		},
	)

	ErrorResponseSchema := "#/components/schemas/ErrorResponse"

	reflector.Spec.Components.WithResponses(
		openapi3.ComponentsResponses{
			MapOfResponseOrRefValues: map[string]openapi3.ResponseOrRef{
				"BadRequest": {
					Response: &openapi3.Response{
						Description: "Bad Request",
						Content: map[string]openapi3.MediaType{
							"application/json": {
								Example: new(any(map[string]any{
									"details": "The request is invalid or missing required parameters.",
								})),
								Schema: &openapi3.SchemaOrRef{
									SchemaReference: &openapi3.SchemaReference{
										Ref: ErrorResponseSchema,
									},
								},
							},
						},
					},
				},
				"NotFound": {
					Response: &openapi3.Response{
						Description: "Not Found",
						Content: map[string]openapi3.MediaType{
							"application/json": {
								Example: new(any(map[string]any{
									"details": "The requested resource was not found.",
								})),
								Schema: &openapi3.SchemaOrRef{
									SchemaReference: &openapi3.SchemaReference{
										Ref: ErrorResponseSchema,
									},
								},
							},
						},
					},
				},
				"Conflict": {
					Response: &openapi3.Response{
						Description: "Conflict",
						Content: map[string]openapi3.MediaType{
							"application/json": {
								Example: new(any(map[string]any{
									"details": "There is a conflict with an existing resource.",
								})),
								Schema: &openapi3.SchemaOrRef{
									SchemaReference: &openapi3.SchemaReference{
										Ref: ErrorResponseSchema,
									},
								},
							},
						},
					},
				},
				"NoContent": {
					Response: &openapi3.Response{
						Description: "No Content",
						Content: map[string]openapi3.MediaType{
							"application/json": {
								Example: new(any(map[string]any{
									"details": "No content to return.",
								})),
								Schema: &openapi3.SchemaOrRef{
									SchemaReference: &openapi3.SchemaReference{
										Ref: ErrorResponseSchema,
									},
								},
							},
						},
					},
				},
				"PreconditionFailed": {
					Response: &openapi3.Response{
						Description: "Precondition Failed",
						Content: map[string]openapi3.MediaType{
							"application/json": {
								Example: new(any(map[string]any{
									"details": "The request is invalid.",
								})),
								Schema: &openapi3.SchemaOrRef{
									SchemaReference: &openapi3.SchemaReference{
										Ref: ErrorResponseSchema,
									},
								},
							},
						},
					},
				},
				"InternalServerError": {
					Response: &openapi3.Response{
						Description: "Internal Server Error",
						Content: map[string]openapi3.MediaType{
							"application/json": {
								Example: new(any(map[string]any{
									"details": "An unexpected error occurred.",
								})),
								Schema: &openapi3.SchemaOrRef{
									SchemaReference: &openapi3.SchemaReference{
										Ref: ErrorResponseSchema,
									},
								},
							},
						},
					},
				},
				"Unauthorized": {
					Response: &openapi3.Response{
						Description: "Unauthorized",
						Content: map[string]openapi3.MediaType{
							"application/json": {
								Example: new(any(map[string]any{
									"details": "Unauthorized access to the resource.",
								})),
								Schema: &openapi3.SchemaOrRef{
									SchemaReference: &openapi3.SchemaReference{
										Ref: ErrorResponseSchema,
									},
								},
							},
						},
					},
				},
				"InsufficientStorage": {
					Response: &openapi3.Response{
						Description: "Insufficient Storage",
						Content: map[string]openapi3.MediaType{
							"application/json": {
								Example: new(any(map[string]any{
									"details": "Insufficient storage to complete the request.",
								})),
								Schema: &openapi3.SchemaOrRef{
									SchemaReference: &openapi3.SchemaReference{
										Ref: ErrorResponseSchema,
									},
								},
							},
						},
					},
				},
				"Forbidden": {
					Response: &openapi3.Response{
						Description: "Forbidden",
						Content: map[string]openapi3.MediaType{
							"application/json": {
								Example: new(any(map[string]any{
									"details": "You do not have permission to access this resource.",
								})),
								Schema: &openapi3.SchemaOrRef{
									SchemaReference: &openapi3.SchemaReference{
										Ref: ErrorResponseSchema,
									},
								},
							},
						},
					},
				},
			},
		},
	)

	// Openapi-go automatically add as prefix the package name. We use this hook
	// to manually remove the pkg prefix.
	reflector.DefaultOptions = append(reflector.DefaultOptions,
		jsonschema.InterceptSchema(func(params jsonschema.InterceptSchemaParams) (stop bool, err error) {

			if params.Value.Type() == reflect.TypeOf(orchestrator.Status("")) {
				params.Schema.WithRef("#/components/schemas/Status")
				return true, nil
			}
			// We treat the orchestrator.ID as a string in the OpenAPI spec.
			if params.Value.Type() == reflect.TypeOf(appid.ID{}) {
				params.Schema.WithType(jsonschema.Type{
					SimpleTypes: new(jsonschema.String),
				})
			}

			if params.Value.Type() == reflect.TypeOf(update.PackageType("")) {
				params.Schema.WithRef("#/components/schemas/PackageType")
				return true, nil
			}
			if params.Value.Type() == reflect.TypeOf(modelsindex.ModelStatus("")) {
				params.Schema.WithRef("#/components/schemas/ModelStatus")
				return true, nil
			}
			return false, nil
		}),
		jsonschema.InterceptDefName(func(t reflect.Type, defaultDefName string) string {
			caser := cases.Title(language.English)
			pkgName := caser.String(path.Base(t.PkgPath()))
			if s, found := strings.CutPrefix(defaultDefName, pkgName); found {
				return s
			}
			return defaultDefName
		}),
	)

	return &Generator{reflector: reflector}
}

func (g *Generator) GetDocs() *openapi3.Spec {
	return g.reflector.Spec
}

type OperationConfig struct {
	OperationId    string
	Method         string
	Path           string
	Parameters     any
	Request        any
	Description    string
	Summary        string
	Tags           []Tag
	PossibleErrors []ErrorResponse

	CustomSuccessResponse *CustomResponseDef
}

type CustomResponseDef struct {
	ContentType   string
	Description   string
	DataStructure any
	StatusCode    int
}
type ErrorResponse struct {
	StatusCode int    `json:"code"`
	Reference  string `json:"message"`
}

func (g *Generator) InitOperations() {

	operations := []OperationConfig{
		{
			OperationId: "DeleteProperty",
			Method:      http.MethodDelete,
			Path:        "/v1/properties/{key}",
			Request: (*struct {
				ID string `path:"key" description:"property key."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: nil,
				Description:   "Successful response",
				StatusCode:    http.StatusNoContent,
			},
			Description: "Delete the property by the provided key.",
			Summary:     "Delete property by key",
			Tags:        []Tag{Property},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusNotFound, Reference: "#/components/responses/NotFound"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "UpdateProperty",
			Method:      http.MethodPut,
			Path:        "/v1/properties/{key}",
			Parameters: (*struct {
				ID string `path:"key" description:"property key."`
			})(nil),
			Request: []byte{},
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/octet-stream",
				DataStructure: []byte{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Update or create a new property.",
			Summary:     "Upsert property",
			Tags:        []Tag{Property},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusNotFound, Reference: "#/components/responses/NotFound"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "GetProperty",
			Method:      http.MethodGet,
			Path:        "/v1/properties/{key}",
			Parameters: (*struct {
				ID string `path:"key" description:"property key."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/octet-stream",
				DataStructure: []byte{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Return a single property by the provided key.",
			Summary:     "Get property by key",
			Tags:        []Tag{Property},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusNotFound, Reference: "#/components/responses/NotFound"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "GetPropertyKeys",
			Method:      http.MethodGet,
			Path:        "/v1/properties",
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: models.PropertyKeysResponse{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Return the list of system properties.",
			Summary:     "Get system properties",
			Tags:        []Tag{Property},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getAppPorts",
			Method:      http.MethodGet,
			Path:        "/v1/apps/{appID}/exposed-ports",
			Request: (*struct {
				ID string `path:"appID" description:"application identifier."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: handlers.AppPortResponse{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Return all ports exposed by the given app.",
			Summary:     "Get app exposed ports",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "deleteApp",
			Method:      http.MethodDelete,
			Path:        "/v1/apps/{id}",
			Request: (*struct {
				ID string `path:"id" description:"application identifier."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				Description: "Successful response",
				StatusCode:  http.StatusOK,
			},
			Description: "Remove the given app and all the resources it created",
			Summary:     "delete the app",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "cloneApp",
			Method:      http.MethodPost,
			Path:        "/v1/apps/{id}/clone",
			Request:     handlers.CloneRequest{},
			Parameters: (*struct {
				ID string `path:"id" description:"application identifier."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: orchestrator.CloneAppResponse{},
				Description:   "Successful response",
				StatusCode:    http.StatusCreated,
			},
			Description: "Clone an existing app or example, in a new one. It is possible to specify the new name and icon.",
			Summary:     "Creates a new app, from another app or example identified by ID.",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusNotFound, Reference: "#/components/responses/NotFound"},
				{StatusCode: http.StatusConflict, Reference: "#/components/responses/Conflict"},
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "stopApp",
			Method:      http.MethodPost,
			Path:        "/v1/apps/{id}/stop",
			Request: (*struct {
				ID string `path:"id" description:"application identifier."`
			})(nil),
			Description: "Stop the application and all it's dependecies. If the app contains a sketch it also remove it from the micro.",
			Summary:     "Stop an existing app/example",
			Tags:        []Tag{ApplicationTag},
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "text/event-stream",
				DataStructure: "",
				Description: `A stream of Server-Sent Events (SSE) that notifies the progress.
The client will receive events formatted as follows:

**Event 'progress'**:
Contains a JSON object with the percentage of completion.
'event: progress'
'data: {"progress":0.25}'

**Event 'message'**:
Contains a JSON object with an informational message.
'event: message'
'data: {"message":"Stopping container..."}'

**Event 'error'**:
Contains a JSON object with the details of an error.
'event: error'
'data: {"code":"INTERNAL_SERVER_ERROR","message":"An error occurred during operation"}'
`,
			},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "startApp",
			Method:      http.MethodPost,
			Path:        "/v1/apps/{id}/start",
			Request: (*struct {
				ID      string `path:"id" description:"application identifier."`
				Verbose bool   `query:"verbose" description:"Return a verbose output. Default is false."`
			})(nil),
			Description: "Start the application and handles all the operation to start any dependecies. If the app contains a sketch it also flash it in the micro.",
			Summary:     "Start an existing app/example",
			Tags:        []Tag{ApplicationTag},
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "text/event-stream",
				DataStructure: "",
				Description: `A stream of Server-Sent Events (SSE) that notifies the progress.
The client will receive events formatted as follows:

**Event 'progress'**:
Contains a JSON object with the percentage of completion.
'event: progress'
'data: {"progress":0.25}'

**Event 'message'**:
Contains a JSON object with an informational message.
'event: message'
'data: {"message":"Starting container..."}'

**Event 'error'**:
Contains a JSON object with the details of an error.
'event: error'
'data: {"code":"INTERNAL_SERVER_ERROR","message":"An error occurred during operation"}'
`,
			},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "editApp",
			Method:      http.MethodPatch,
			Path:        "/v1/apps/{id}",
			Request:     handlers.EditRequest{},
			Parameters: (*struct {
				ID string `path:"id" description:"application identifier."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: orchestrator.AppDetailedInfo{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Edit the given application. Is it possible to modify the default status, to add/remove/update bricks and bricks variables.",
			Summary:     "Update App Details",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getAppDetails",
			Method:      http.MethodGet,
			Path:        "/v1/apps/{id}",
			Request: (*struct {
				ID string `path:"id" description:"application identifier."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: orchestrator.AppDetailedInfo{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Return all the detail for the given app",
			Summary:     "Get app/example detail",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getAppsEvents",
			Method:      http.MethodGet,
			Path:        "/v1/apps/events",
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "text/event-stream",
				DataStructure: orchestrator.LogMessage{},
			},
			Description: `A stream of Server-Sent Events (SSE) that notifies the apps status.
The client will receive events formatted as follows:

**Event 'app'**:
Contains a JSON object with an informational message.
'event: app'
'data: {"id":"dXNlcjpleGFtcG","name":"example-app-for-status-events","description":"My app description","icon":"💻","status":"running","example":false,"default":false}'

**Event 'error'**:
Contains a JSON object with the details of an error.
'event: error'
'data: {"code":"INTERNAL_SERVER_ERROR","message":"An error occurred during operation"}'
`,
			Summary: "Get application events",
			Tags:    []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "importApp",
			Method:      http.MethodPost,
			Path:        "/v1/apps/import",
			Parameters:  nil,
			Request: (*struct {
				File []byte `form:"file" description:"The ZIP archive. Must contain app.yaml (with a valid 'name') and python/main.py. The app folder name will be calculated from the app name." validate:"required"`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType: "application/json",
				StatusCode:  http.StatusCreated,
				DataStructure: struct {
					ID string `json:"id" description:"The Base64 encoded identifier of the imported application."`
				}{},
				Description: "Application imported successfully.",
			},
			Description: "Imports a new application from a ZIP file. The system extracts the archive, validates the app.yaml manifest, sanitizes the name, and returns the ID in Base64.",
			Summary:     "Imports an app from ZIP",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusConflict, Reference: "#/components/responses/Conflict"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "exportApp",
			Method:      http.MethodGet,
			Path:        "/v1/apps/{id}/export",
			Request: (*struct {
				ID string `path:"id" description:"application identifier."`
			})(nil),
			Parameters: (*struct {
				IncludeData bool `query:"include_data" description:"If true, the exported archive will include the 'data' directory. Default is false."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/zip",
				DataStructure: []byte{},
				Description:   "The ZIP archive containing the application structure.",
				StatusCode:    http.StatusOK,
			},
			Description: "Exports the application folder structure as a ZIP file.",
			Summary:     "Exports an app as ZIP",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusNotFound, Reference: "#/components/responses/NotFound"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getAppEvents",
			Method:      http.MethodGet,
			Path:        "/v1/apps/{id}/events",
			Request: (*struct {
				ID string `path:"id" description:"application identifier."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "text/event-stream",
				DataStructure: orchestrator.LogMessage{},
			},
			Description: "Returns events for a specific app ",
			Summary:     "Get application events",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getAppLogs",
			Method:      http.MethodGet,
			Path:        "/v1/apps/{id}/logs",
			Request: (*struct {
				ID       string `path:"id" description:"application identifier."`
				Filter   string `query:"filter"`
				Tail     int    `query:"tail"`
				Nofollow bool   `query:"nofollow"`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "text/event-stream",
				DataStructure: orchestrator.LogMessage{},
			},
			Description: "Obtain a ServerSentEvnt stream of logs. It is possible to apply different filters.",
			Summary:     "Get the logs of a running app",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "createApp",
			Method:      http.MethodPost,
			Path:        "/v1/apps",
			Request:     handlers.CreateAppRequest{},
			Parameters: (*struct {
				SkipSketch bool `query:"skip-sketch" description:"If true, the app will not be created with the sketch part."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: orchestrator.CreateAppResponse{},
				Description:   "Successful response",
				StatusCode:    http.StatusCreated,
			},
			Description: "Creates a new app in the default app location.",
			Summary:     "Creates a new app",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusConflict, Reference: "#/components/responses/Conflict"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getApps",
			Method:      http.MethodGet,
			Path:        "/v1/apps",
			Request:     (*orchestrator.ListAppRequest)(nil),
			Parameters: (*struct {
				Filter string              `query:"filter" description:"Filters apps by apps,examples,default"`
				Status orchestrator.Status `query:"status" description:"Filters applications by status"`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: handlers.AppListResponse{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Returns a list of all apps, and example present. It is also possible to apply different filters.",
			Summary:     "Get a list of installed apps/examples",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getBrickDetails",
			Method:      http.MethodGet,
			Path:        "/v1/bricks/{id}",
			Request: (*struct {
				ID string `path:"id" description:"brick identifier."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: bricks.BrickDetailsResult{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Returns a detailed list of property associated to the given brick.",
			Summary:     "Detail of a brick",
			Tags:        []Tag{BrickTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusNotFound, Reference: "#/components/responses/NotFound"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"}},
		},
		{
			OperationId: "getBricks",
			Method:      http.MethodGet,
			Path:        "/v1/bricks",
			Request:     nil,
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: bricks.BrickListResult{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Returns all the existing bricks. Bricks that are ready to use are marked as installed.",
			Summary:     "Get a list of available bricks",
			Tags:        []Tag{BrickTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getConfig",
			Method:      http.MethodGet,
			Path:        "/v1/config",
			Request:     nil,
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: orchestrator.ConfigResponse{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "returns information about current directory configuration used by the app",
			Summary:     "returns application configuration",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getExamples",
			Method:      http.MethodGet,
			Path:        "/v1/examples",
			Request:     nil,
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: orchestrator.ExampleResponse{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "returns the example structure for rendering",
			Summary:     "returns the example structure",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getVersions",
			Method:      http.MethodGet,
			Path:        "/v1/version",
			Request:     nil,
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: handlers.VersionResponse{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "returns the application current version",
			Summary:     "application version",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getAIModels",
			Method:      http.MethodGet,
			Path:        "/v1/models",
			Request: (*struct {
				Bricks string `query:"bricks" description:"Filter models by bricks. If not specified, all models are returned."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: orchestrator.AIModelsListResult{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Returns the list of AI models available in the system. It is possible to filter the models by bricks.",
			Summary:     "Get a list of available AI models",
			Tags:        []Tag{AIModelsTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "deleteAIModel",
			Method:      http.MethodDelete,
			Path:        "/v1/models/{id}",
			Request: (*struct {
				ID    string `path:"id" description:"AI model identifier."`
				Force bool   `query:"force" description:"If true, deletes the model even if referenced by apps."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				StatusCode:  http.StatusNoContent,
				Description: "AI model successfully deleted",
			},
			Description: "Deletes a specific AI model. By default, fails if referenced by apps unless force=true is provided.",
			Summary:     "Delete an AI model",
			Tags:        []Tag{AIModelsTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusNotFound, Reference: "#/components/responses/NotFound"},
				{StatusCode: http.StatusConflict, Reference: "#/components/responses/Conflict"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getAIModelDetails",
			Method:      http.MethodGet,
			Path:        "/v1/models/{id}",
			Request: (*struct {
				ID string `path:"id" description:"AI model identifier. An id no internal model list entry declares carries the repository path the model was downloaded from, so its slashes must be percent-encoded." example:"llamacpp:unsloth%2FSmolLM2-135M-Instruct-GGUF%2FSmolLM2-135M-Instruct-Q4_K_M"`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: orchestrator.AIModelItem{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Returns the details of a specific AI model.",
			Summary:     "Get AI model details",
			Tags:        []Tag{AIModelsTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "installEIModel",
			Method:      http.MethodPut,
			Path:        "/v1/models/ei/projects/{projectID}",
			Request: (*struct {
				ProjectID int    `path:"projectID" description:"Edge Impulse project ID" example:"123456" required:"true"`
				ImpulseID int    `json:"impulse_id" description:"Edge Impulse impulse ID" example:"1" required:"true"`
				PrjApiKey string `header:"x-api-key" description:"Edge Impulse project API key" example:"your_edge_impulse_api_token" required:"true"`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: orchestrator.AIModelItem{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Download and install a custom Edge Impulse AI model using the provided project API key.",
			Summary:     "Download and install a custom Edge Impulse AI model",
			Tags:        []Tag{AIModelsTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
				{StatusCode: http.StatusUnauthorized, Reference: "#/components/responses/Unauthorized"},
				{StatusCode: http.StatusForbidden, Reference: "#/components/responses/Forbidden"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInsufficientStorage, Reference: "#/components/responses/InsufficientStorage"},
			},
		},
		{
			OperationId: "installModel",
			Method:      http.MethodPut,
			// Documented as {id} like the GET and DELETE on the same path: OpenAPI treats
			// two templated paths of the same hierarchy with different names as identical,
			// so {modelID} here would collide with them.
			Path: "/v1/models/{id}",
			Parameters: (*struct {
				ModelID string `path:"id" description:"A model id the internal model list declares, or a Hugging Face source to fetch one from: a file URL, or the compact \"[type:]repo:quantization[:mmproj_quantization]\" key. Percent-encode the slashes." example:"llamacpp:unsloth%2FSmolLM2-135M-Instruct-GGUF:Q4_K_M"`
			})(nil),
			Request: (*struct {
				MmprojURL string `json:"model_mmproj_url" description:"Multimodal projection file for a vision model, when the source is a file URL. A compact key names it inline instead, and a declared model needs no body at all." example:"https://huggingface.co/unsloth/SmolLM2-135M-Instruct-GGUF/blob/main/mmproj-F16.gguf"`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "text/event-stream",
				DataStructure: "",
				Description: `A stream of Server-Sent Events (SSE) reporting the download.

**Event 'message'**:
A line of progress information from the handler.
'event: message'
'data: {"message":"Downloading to: /models/llamacpp/unsloth/SmolLM2-135M-Instruct-GGUF"}'

**Event 'progress'**:
Bytes transferred for the file being downloaded.
'event: progress'
'data: {"name":"SmolLM2-135M-Instruct-Q4_K_M.gguf","current":75876627,"total":105454144,"progress":71.95}'

**Event 'done'**:
The installed model. A declared model is described by its own entry. For a Hugging Face
source the downloader assigns the id from the file that arrives, so it is not known to the
caller before this event, and it is qualified by the repository the file came from.
'event: done'
'data: {"id":"llamacpp:unsloth/SmolLM2-135M-Instruct-GGUF/SmolLM2-135M-Instruct-Q4_K_M","name":"unsloth/SmolLM2-135M-Instruct-GGUF/SmolLM2-135M-Instruct-Q4_K_M","status":"installed"}'

**Event 'error'**:
Contains a JSON object with the details of an error.
'event: error'
'data: {"code":"INTERNAL_SERVER_ERROR","message":"An error occurred during operation"}'
`,
			},
			Description: `Download and install an AI model, streaming the progress as Server-Sent Events.

The path names either a model the internal model list declares, or a Hugging Face source to fetch one from - a file URL, or the compact "[type:]repo:quantization[:mmproj_quantization]" key. Percent-encode the slashes.

The internal model list decides which it is: an id it declares is installed as a declared model, and anything else is handed to the Hugging Face downloader. That decision reads the model list alone, so the request starts no container before the download. A path that is neither a declared id nor a source the Hub resolves is therefore reported by the downloader, as a stream error saying the repository does not exist, rather than as a 404.

Nothing is checked before the download starts, and the request is idempotent. A model already on disk is not transferred again: the downloader reports the one it finds and the stream ends in "done" with its item, the same answer a fresh install gives. A declared model whose declaration names no handler, pre-loaded or custom, is installed by that declaration alone and answers "done" at once. A Hugging Face source has no identifier until the downloader assigns one from the file that arrives, so the id in the "done" event is not the one in the path.

Installing from a Hugging Face source requires a models-downloader image that reports "model_id" on its download events (arduino/app-bricks-py#415 and arduino/app-bricks-py#416). An older image installs the model but names nothing, and the request answers 500 rather than guess at an id no later request would resolve.`,
			Summary: "Download and install an AI model",
			Tags:    []Tag{AIModelsTag},
			// Neither 404 nor 409: no listing runs, so an id nothing declares cannot be
			// told from one that is merely absent, and an installed model is answered
			// rather than refused.
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
				{StatusCode: http.StatusInsufficientStorage, Reference: "#/components/responses/InsufficientStorage"},
			},
		},
		{
			OperationId: "getSystemResources",
			Method:      http.MethodGet,
			Path:        "/v1/system/resources",
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "text/event-stream",
				DataStructure: "",
				Description: `A stream of Server-Sent Events (SSE) that notifies the stats.
The client will receive events formatted as follows:

**Event 'cpu'**:
Contains a JSON object with the CPU information.
'event: stats'
'data: {"used_percent": 0.25}'

**Event 'mem'**:
Contains a JSON object with the memory information.
'event: mem'
'data: {"used": 1024, "total": 2048}'

**Event 'disk'**:
Contains a JSON object with the disk information.
'event: disk'
'data: {"path":"/", "used": 512, "total": 1024}'

**Event 'error'**:
Contains a JSON object with the details of an error.
'event: error'
'data: {"code":"INTERNAL_SERVER_ERROR","message":"An error occurred during operation"}'
`,
			},
			Description: "Returns the system resources usage, such as memory, disk and CPU.",
			Summary:     "Get system resources usage",
			Tags:        []Tag{SystemTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "checkUpdate",
			Method:      http.MethodGet,
			Path:        "/v1/system/update/check",
			Parameters: (*struct {
				OnlyArduino bool `query:"only-arduino" description:"If true, check only for Arduino packages that require an upgrade. Default is false."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: handlers.UpdateCheckResult{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Returns the details of packages to be upgraded.",
			Summary:     "Get the packages that requires an upgrade",
			Tags:        []Tag{SystemTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusNoContent, Reference: "#/components/responses/NoContent"},
			},
		},
		{
			OperationId: "applyUpdate",
			Method:      http.MethodPut,
			Path:        "/v1/system/update/apply",
			Parameters: (*struct {
				OnlyArduino bool `query:"only-arduino" description:"If true, upgrade only the Arduino packages that require an upgrade. Default is false."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				Description: "Successful response",
				StatusCode:  http.StatusOK,
			},
			Description: "Start the upgrade process.",
			Summary:     "Start the upgrade process in background",
			Tags:        []Tag{SystemTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusConflict, Reference: "#/components/responses/Conflict"},
				{StatusCode: http.StatusNoContent, Reference: "#/components/responses/NoContent"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "eventsUpdate",
			Method:      http.MethodGet,
			Path:        "/v1/system/update/events",
			Request:     nil,
			Description: "Returns the events of current update process.",
			Summary:     "SSE stream of the update process",
			Tags:        []Tag{SystemTag},
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "text/event-stream",
				DataStructure: "",
				Description: `A stream of Server-Sent Events (SSE) that notifies the progress of the update process.
The client will receive events formatted as follows:

**Event 'log'**:
Contains a log message of the apt upgrade command.
'event: log'
'data: "updating package: 0.25"'

**Event 'starting'**:
Contains a string with the message that a step of the upgrade process is starting.
'event: starting'
'data: Upgrade is starting'

**Event 'progress'**:
Contains a JSON object with the overall completion percentage of the update process,
from 0 to 100, and the step it is currently executing.
'event: progress'
'data: {"step":"docker images download","progress":44}'

**Event 'restarting'**:
Contains a string with the message that the upgrade is completed and the system is restarting.
'event: restarting'
'data: Upgrade completed. Restarting'

**Event 'error'**:
Contains a JSON object with the details of an error.
'event: error'
'data: {"code":"internal_service_err","message":"An error occurred during operation"}'
`,
			},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getAppBrickInstances",
			Method:      http.MethodGet,
			Path:        "/v1/apps/{appID}/bricks",
			Parameters: (*struct {
				ID string `path:"appID" description:"application identifier."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: bricks.AppBrickInstancesResult{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Get the list of brick instances for a specific app.",
			Summary:     "Get brick instances for an app",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "createAppLocalBrick",
			Method:      http.MethodPost,
			Path:        "/v1/apps/{appID}/bricks",
			Parameters: (*struct {
				ID string `path:"appID" description:"application identifier."`
			})(nil),
			Request: handlers.AppLocalBrickCreateRequest{},
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: handlers.AppLocalBrickCreateResponse{},
				Description:   "Successful response",
				StatusCode:    http.StatusCreated,
			},
			Description: "Create a new local brick for an app.",
			Summary:     "Create a new local brick for an app",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusConflict, Reference: "#/components/responses/Conflict"},
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "getAppBrickInstanceByBrickID",
			Method:      http.MethodGet,
			Path:        "/v1/apps/{appID}/bricks/{brickID}",
			Parameters: (*struct {
				ID      string `path:"appID" description:"application identifier."`
				BrickID string `path:"brickID" description:"brick identifier."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: bricks.BrickInstance{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Get a specific brick instance for an app by its ID.",
			Summary:     "Get a specific brick instance by ID",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "upsertAppBrickInstance",
			Method:      http.MethodPut,
			Path:        "/v1/apps/{appID}/bricks/{brickID}",
			Parameters: (*struct {
				ID      string `path:"appID" description:"application identifier."`
				BrickID string `path:"brickID" description:"brick identifier."`
			})(nil),
			Request: bricks.BrickCreateUpdateRequest{},
			CustomSuccessResponse: &CustomResponseDef{
				Description: "Successful response",
				StatusCode:  http.StatusOK,
			},
			Description: "Upsert a brick instance for an app. If the instance does not exist, it will be created. If it exists, it will be updated.",
			Summary:     "Upsert a brick instance for an app",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "updateAppBrickInstance",
			Method:      http.MethodPatch,
			Path:        "/v1/apps/{appID}/bricks/{brickID}",
			Parameters: (*struct {
				ID      string `path:"appID" description:"application identifier."`
				BrickID string `path:"brickID" description:"brick identifier."`
			})(nil),
			Request: bricks.BrickCreateUpdateRequest{},
			CustomSuccessResponse: &CustomResponseDef{
				Description: "Successful response",
				StatusCode:  http.StatusOK,
			},
			Description: "Update a brick instance for an app. It update/add only the provided fields.",
			Summary:     "Update a brick instance for an app",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "deleteAppBrickInstance",
			Method:      http.MethodDelete,
			Path:        "/v1/apps/{appID}/bricks/{brickID}",
			Parameters: (*struct {
				ID      string `path:"appID" description:"application identifier."`
				BrickID string `path:"brickID" description:"brick identifier."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				Description: "Successful response",
				StatusCode:  http.StatusOK,
			},
			Description: "Delete a brick instance for an app. It will remove the brick instance from the app.",
			Summary:     "Delete a brick instance for an app",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "renameAppLocalBrick",
			Method:      http.MethodPost,
			Path:        "/v1/apps/{appID}/bricks/{brickID}/rename",
			Parameters: (*struct {
				ID      string `path:"appID" description:"application identifier."`
				BrickID string `path:"brickID" description:"brick identifier."`
			})(nil),
			Request: handlers.AppLocalBrickRenameRequest{},
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: bricks.LocalBrickRenameResult{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Rename a local brick. Changes the brick's ID and folder name derived from the new name. Only local bricks can be renamed.",
			Summary:     "Rename a local brick",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusNotFound, Reference: "#/components/responses/NotFound"},
				{StatusCode: http.StatusConflict, Reference: "#/components/responses/Conflict"},
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "listLibraries",
			Method:      http.MethodGet,
			Path:        "/v1/libraries",
			Parameters: (*struct {
				Search       string `query:"search" description:"Search term to filter libraries by name, sentence, paragraph."`
				Architecture string `query:"architecture" description:"Filter libraries by target architecture"`
				Platform     string `query:"platform" description:"Filter libraries by platform"`
				Sort         string `query:"sort" description:"Sort order for the results" enum:"stars_asc,stars_desc,forks_asc,forks_desc,recent_asc,recent_desc" default:"stars_desc"`
				Page         int    `query:"page" description:"Page number for pagination" minimum:"1" default:"1"`
				Limit        int    `query:"limit" description:"Number of results per page" minimum:"1" maximum:"1000" default:"20"`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: handlers.LibraryListResponse{},
				Description:   "Successful response with library search results",
				StatusCode:    http.StatusOK,
			},
			Description: "Search for Arduino libraries in the registry with various filters.",
			Summary:     "Search Arduino libraries",
			Tags:        []Tag{LibrariesTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "appSketchAddLibrary",
			Method:      http.MethodPut,
			Path:        "/v1/apps/{appID}/sketch/libraries/{libRef}",
			Parameters: (*struct {
				ID              string `path:"appID" description:"application identifier."`
				LibRef          string `path:"libRef" description:"library reference (\"LibraryName\" or \"LibraryName@Version\")."`
				AddDependencies bool   `query:"add_deps" description:"if set to \"true\", the library's dependencies will be added as well."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: handlers.SketchAddLibraryResponse{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Adds a library to the App' sketch. The library will be added to the sketch project file. The dependencies of the library may be optionally added as well.",
			Summary:     "Adds a library to the App' sketch.",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "appSketchRemoveLibrary",
			Method:      http.MethodDelete,
			Path:        "/v1/apps/{appID}/sketch/libraries/{libRef}",
			Parameters: (*struct {
				ID                 string `path:"appID" description:"application identifier."`
				LibRef             string `path:"libRef" description:"library reference (\"LibraryName\" or \"LibraryName@Version\")."`
				RemoveDependencies bool   `query:"remove_deps" description:"if set to \"true\", the library's dependencies will be removed as well if not needed anymore."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: handlers.SketchRemoveLibraryResponse{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Removes a library from the App' sketch. The library will be removed from the sketch project file.",
			Summary:     "Removes a library from the App' sketch.",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
		{
			OperationId: "appSketchListLibraries",
			Method:      http.MethodGet,
			Path:        "/v1/apps/{appID}/sketch/libraries",
			Parameters: (*struct {
				ID string `path:"appID" description:"application identifier."`
			})(nil),
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: handlers.SketchListLibraryResponse{},
				Description:   "Successful response",
				StatusCode:    http.StatusOK,
			},
			Description: "Lists the libraries used in the App' sketch.",
			Summary:     "Lists the libraries used in the App' sketch.",
			Tags:        []Tag{ApplicationTag},
			PossibleErrors: []ErrorResponse{
				{StatusCode: http.StatusPreconditionFailed, Reference: "#/components/responses/PreconditionFailed"},
				{StatusCode: http.StatusBadRequest, Reference: "#/components/responses/BadRequest"},
				{StatusCode: http.StatusInternalServerError, Reference: "#/components/responses/InternalServerError"},
			},
		},
	}

	for _, op := range operations {
		if err := g.AddOperation(op); err != nil {
			slog.Error(
				"failed to register OpenApi operation",
				"path", op.Path,
				"method", op.Method,
				"error", err,
			)
		}
	}

	g.reflector.Spec.WithTags(
		f.Map(validTags, func(t Tag) openapi3.Tag {
			return openapi3.Tag{Name: string(t)}
		})...,
	)
}
func (g *Generator) AddOperation(config OperationConfig) error {
	opCtx, err := g.reflector.NewOperationContext(config.Method, config.Path)
	if err != nil {
		return err
	}
	opCtx.SetDescription(config.Description)
	opCtx.SetTags(f.Map(config.Tags, func(t Tag) string { return string(t) })...)
	opCtx.SetSummary(config.Summary)
	opCtx.AddReqStructure(config.Request)
	opCtx.SetID(config.OperationId)

	if config.Parameters != nil {
		opCtx.AddReqStructure(config.Parameters)
	}

	opCtx.AddRespStructure(config.CustomSuccessResponse.DataStructure, func(cu *openapi.ContentUnit) {
		cu.HTTPStatus = config.CustomSuccessResponse.StatusCode
		cu.ContentType = config.CustomSuccessResponse.ContentType
		cu.Description = config.CustomSuccessResponse.Description

		if cu.ContentType == "application/zip" {
			cu.Customize = func(cor openapi.ContentOrReference) {
				respOrRef, ok := cor.(*openapi3.ResponseOrRef)
				if !ok || respOrRef.Response == nil {
					return
				}
				content, exists := respOrRef.Response.Content[cu.ContentType]
				if !exists {
					return
				}
				if content.Schema != nil && content.Schema.Schema != nil {
					content.Schema.Schema.Type = new(openapi3.SchemaTypeString)
					content.Schema.Schema.Format = new("binary")
				}
			}
		}
	})
	for _, e := range config.PossibleErrors {
		opCtx.AddRespStructure(e, func(cu *openapi.ContentUnit) {
			cu.Customize = func(cor openapi.ContentOrReference) {
				cor.SetReference(e.Reference)
			}
			cu.HTTPStatus = e.StatusCode
		})
	}

	err = g.reflector.AddOperation(opCtx)
	if err != nil {
		return err
	}
	return nil
}
