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
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/update"
)

type Tag string

const (
	ApplicationTag Tag = "Application"
	BrickTag       Tag = "Brick"
	AIModelsTag    Tag = "AIModels"
	SystemTag      Tag = "System"
)

var validTags = []Tag{ApplicationTag, BrickTag, AIModelsTag, SystemTag}

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
	reflector.Spec.Components.Schemas.WithMapOfSchemaOrRefValuesItem(
		"PackageType",
		openapi3.SchemaOrRef{
			Schema: &openapi3.Schema{
				UniqueItems: f.Ptr(true),
				Enum:        f.Map(update.PackageType("").AllowedStatuses(), func(v update.PackageType) interface{} { return v }),
				Type:        f.Ptr(openapi3.SchemaTypeString),
				Description: f.Ptr("Package type"),
				ReflectType: reflect.TypeOf(update.PackageType("")),
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
								Example: f.Ptr(interface{}(map[string]interface{}{
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
								Example: f.Ptr(interface{}(map[string]interface{}{
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
								Example: f.Ptr(interface{}(map[string]interface{}{
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
								Example: f.Ptr(interface{}(map[string]interface{}{
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
								Example: f.Ptr(interface{}(map[string]interface{}{
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
			if params.Value.Type() == reflect.TypeOf(orchestrator.ID{}) {
				params.Schema.WithType(jsonschema.Type{
					SimpleTypes: f.Ptr(jsonschema.String),
				})
			}

			if params.Value.Type() == reflect.TypeOf(update.PackageType("")) {
				params.Schema.WithRef("#/components/schemas/PackageType")
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
				SkipPython bool `query:"skip-python" description:"If true, the app will not be created with the python part."`
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
			OperationId: "getAIModelDetails",
			Method:      http.MethodGet,
			Path:        "/v1/models/{id}",
			Request: (*struct {
				ID string `path:"id" description:"AI model identifier."`
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
