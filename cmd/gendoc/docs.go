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
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

type Tag string

const (
	ApplicationTag Tag = "Application"
	BrickTag       Tag = "Brick"
)

var validTags = []Tag{ApplicationTag, BrickTag}

type Generator struct {
	reflector *openapi3.Reflector
}

func NewOpenApiGenerator(version string) *Generator {
	reflector := openapi3.NewReflector()
	reflector.Spec.Info.WithTitle("Arduino-App-Cli").WithVersion(version)
	reflector.Spec.Info.WithDescription("API specification for the MonzaImola Orchestrator")
	reflector.Spec.Servers = append(reflector.Spec.Servers, openapi3.Server{
		URL:         "http://localhost:8080",
		Description: f.Ptr("local server"),
	})

	reflector.Spec.Components = &openapi3.Components{}
	reflector.Spec.Components.Schemas = &openapi3.ComponentsSchemas{}
	reflector.Spec.Components.Schemas.WithMapOfSchemaOrRefValuesItem(
		"Status",
		openapi3.SchemaOrRef{
			Schema: &openapi3.Schema{
				UniqueItems: f.Ptr(true),
				Enum:        f.Map(orchestrator.Status("").AllowedStatuses(), func(v orchestrator.Status) interface{} { return v }),
				Type:        f.Ptr(openapi3.SchemaTypeString),
				Description: f.Ptr("Application status"),
				ReflectType: reflect.TypeOf(orchestrator.Status("")),
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
								Example: f.Ptr(interface{}(map[string]interface{}{
									"code":    400,
									"message": "The request is invalid or missing required parameters.",
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
								Example: f.Ptr(interface{}(map[string]interface{}{
									"code":    404,
									"message": "The requested resource was not found.",
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
								Example: f.Ptr(interface{}(map[string]interface{}{
									"code":    409,
									"message": "There is a conflict with an existing resource.",
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
								Example: f.Ptr(interface{}(map[string]interface{}{
									"code":    412,
									"message": "The request is invalid.",
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
								Example: f.Ptr(interface{}(map[string]interface{}{
									"code":    int64(500),
									"message": "An unexpected error occurred.",
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
	Parameters     interface{}
	Request        interface{}
	Description    string
	Summary        string
	Tags           []Tag
	PossibleErrors []ErrorResponse

	CustomSuccessResponse *CustomResponseDef
}

type CustomResponseDef struct {
	ContentType   string
	Description   string
	DataStructure interface{}
	StatusCode    int
}
type ErrorResponse struct {
	StatusCode int    `json:"code"`
	Reference  string `json:"message"`
}

func (g *Generator) InitOperations() {

	operations := []OperationConfig{
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
'data: {"code":"internal_service_err","message":"An error occurred during operation"}'
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
				ID string `path:"id" description:"application identifier."`
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
'data: {"code":"internal_service_err","message":"An error occurred during operation"}'
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
				Description: "Successful response",
				StatusCode:  http.StatusOK,
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
			Summary:     "Get application events ",
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
				SkipPython bool `query:"skipPython" description:"If true, the app will not be created with the python part."`
				SkipSketch bool `query:"skipSketch" description:"If true, the app will not be created with the sketch part."`
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
			CustomSuccessResponse: &CustomResponseDef{
				ContentType:   "application/json",
				DataStructure: orchestrator.ListAppResult{},
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
				DataStructure: orchestrator.BrickDetailsResult{},
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
				DataStructure: orchestrator.BrickListResult{},
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
