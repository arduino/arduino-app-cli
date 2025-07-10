package bricksindex

import (
	"testing"

	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
)

func TestBricksIndex(t *testing.T) {
	x := `bricks:
- id: arduino:image_classification
  name: Image Classification
  description: "Brick for image classification using a pre-trained model. It processes\
    \ images and returns the predicted class label and confidence score.\nBrick is\
    \ designed to work with pre-trained models provided by framework or with custom\
    \ image classification models trained on Edge Impulse platform. \n"
  require_container: true
  require_model: true
  ports: []
  model_name: mobilenet-image-classification
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /opt/models/ei/
    description: path to the custom model directory
  - name: EI_CLASSIFICATION_MODEL
    default_value: /models/ootb/ei/mobilenet-v2-224px.eim
    description: path to the model file
- id: arduino:camera_scanner
  name: Camera Scanner
  description: Scans a camera for barcodes and QR codes
  require_container: false
  require_model: false
  ports: []
- id: arduino:streamlit_ui
  name: Streamlit UI
  description: A simplified user interface based on Streamlit and Python.
  require_container: false
  require_model: false
  ports:
  - 7000
- id: arduino:keyword_spotter
  name: Keyword Spotter
  description: 'Brick for keyword spotting using a pre-trained model. It processes
    audio input to detect specific keywords or phrases.

    Brick is designed to work with pre-trained models provided by framework or with
    custom audio classification models trained on Edge Impulse platform.

    '
  require_container: true
  require_model: true
  ports: []
  model_name: keyword-spotting-hello-world
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /opt/models/ei/
    description: path to the custom model directory
  - name: EI_KEYWORK_SPOTTING_MODEL
    default_value: /models/ootb/ei/keyword-spotting-hello-world.eim
    description: path to the model file
- id: arduino:mqtt
  name: MQTT Connector
  description: MQTT connector module. Acts as a client for receiving and publishing
    messages to an MQTT broker.
  require_container: false
  require_model: false
  ports: []
- id: arduino:web_ui
  name: Web UI
  description: A user interface based on HTML and JavaScript that can rely on additional
    APIs and a WebSocket exposed by a web server.
  require_container: false
  require_model: false
  ports:
  - 7000
- id: arduino:dbstorage_tsstore
  name: Database Storage - Time Series Store
  description: Simplified time series database storage layer for Arduino sensor samples
    built on top of InfluxDB.
  require_container: true
  require_model: false
  ports: []
  variables:
  - name: APP_HOME
    default_value: .
  - name: DB_PASSWORD
    default_value: Arduino15
    description: Database password
  - name: DB_USERNAME
    default_value: admin
    description: Edge Impulse project API key
  - name: INFLUXDB_ADMIN_TOKEN
    default_value: 392edbf2-b8a2-481f-979d-3f188b2c05f0
    description: InfluxDB admin token
- id: arduino:dbstorage_sqlstore
  name: Database Storage - SQLStore
  description: Simplified database storage layer for Arduino sensor data using SQLite
    local database.
  require_container: false
  require_model: false
  ports: []
- id: arduino:object_detection
  name: Object Detection
  description: "Brick for object detection using a pre-trained model. It processes\
    \ images and returns the predicted class label, bounding-boxes and confidence\
    \ score.\nBrick is designed to work with pre-trained models provided by framework\
    \ or with custom object detection models trained on Edge Impulse platform. \n"
  require_container: true
  require_model: true
  ports: []
  model_name: yolox-object-detection
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /opt/models/ei/
    description: path to the custom model directory
  - name: EI_OBJ_DETECTION_MODEL
    default_value: /models/ootb/ei/yolo-x-nano.eim
    description: path to the model file
- id: arduino:weather_forecast
  name: Weather Forecast
  description: Online weather forecast module for Arduino using open-meteo.com geolocation
    and weather APIs. Requires an internet connection.
  require_container: false
  require_model: false
  ports: []
- id: arduino:visual_anomaly_detection
  name: Visual Anomaly Detection
  description: "Brick for visual anomaly detection using a pre-trained model. It processes\
    \ images and returns detected anomalies and bounding-boxes.\nBrick is designed\
    \ to work with pre-trained models provided by framework or with custom object\
    \ detection models trained on Edge Impulse platform. \n"
  require_container: true
  require_model: true
  ports: []
  model_name: concreate-crack-anomaly-detection
  variables:
  - name: CUSTOM_MODEL_PATH
    default_value: /opt/models/ei/
    description: path to the custom model directory
  - name: EI_V_ANOMALY_DETECTION_MODEL
    default_value: /models/ootb/ei/concrete-crack-anomaly-detection.eim
    description: path to the model file
`

	var index BricksIndex
	err := yaml.Unmarshal([]byte(x), &index)
	require.NoError(t, err)
	require.Len(t, index.Bricks, 11)

	// Check if ports are correctly set
	b, found := index.FindBrickByID("arduino:web_ui")
	require.True(t, found)
	require.Equal(t, []string{"7000"}, b.Ports)

	// Check if variables are correctly set
	b, found = index.FindBrickByID("arduino:image_classification")
	require.True(t, found)
	require.Equal(t, "Image Classification", b.Name)
	require.Equal(t, "mobilenet-image-classification", b.ModelName)
	require.True(t, b.RequireModel)
	require.Len(t, b.Variables, 2)
	require.Equal(t, "CUSTOM_MODEL_PATH", b.Variables[0].Name)
	require.Equal(t, "/opt/models/ei/", b.Variables[0].DefaultValue)
	require.Equal(t, "path to the custom model directory", b.Variables[0].Description)
	require.Equal(t, "EI_CLASSIFICATION_MODEL", b.Variables[1].Name)
	require.Equal(t, "/models/ootb/ei/mobilenet-v2-224px.eim", b.Variables[1].DefaultValue)
	require.Equal(t, "path to the model file", b.Variables[1].Description)
	require.False(t, b.Variables[0].IsRequired())
	require.False(t, b.Variables[1].IsRequired())
}
