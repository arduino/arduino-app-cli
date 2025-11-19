Ths is a possible implementation of AI model support in AppLab.

# Models
In AI, a model is a trained system able to recognize patterns and/or make predictions.
In AppLab, a model is an autonomous entity that can exist independently of brick utilization.

The approach proposed for the storage of custom models is to mirror the library architecture.
Custom models will reside in a tree structure on the board, where associated configuration variables are defined within a YAML configuration file.

A user can create a new model by creating the model folder structure. Custom metadata can be added to the model's YAML file.

## Example of models definition
Models are defined by the following folder structure. This could be a package to be installed on the board.
```
models/
    README.md
    conf.yaml
	metadata.yaml
    model_name/
		build#1
		build#2
```

*NOTE:* We need to discuss how to handle different builds of the same model. In this case, we must ensure we reference the specific build to be run and manage the models' and model definitions' state and deletion accordingly.

*NOTE:* To be discussed how to map Edge Impulse in an AppLab model. See Edge Impulse section below.

The ```conf.yaml``` file contains generic model information. The application expects this data to be available for all models.
```
   runner: brick
   name : "Glass breaking classifier"
   model_labels:
     - "Background"
     - "Glass_Breaking"
```

The ```metadata.yaml``` file contains model-specific metadata.
```
     source: "edgeimpulse"
     ei-project-id: 749446
     source-model-id: "glass-breaking"
     source-model-url: "https://studio.edgeimpulse.com/studio/749446"
```
NOTE: Should we have a metadata section for each category?

## Category
### Category definition
In AI, a category is a common way to classify AI models, indicating the type of data they process or generate. Examples include audio, image, video, and text model categories.

In AppLab, a category is defined as a keyword used to connect models to bricks. It defines which models can be attached to which bricks. Both a model and a brick have a list of categories.

### Use Case (Linking Models to Bricks)
Use Case (link the Figma here): In the brick configuration, we can choose the model to be used with the brick. The provided list is a filtered list of all models where the intersection of the model's category list and the brick's category list is not empty.

**We assume here that every model with a given category is able to work with any provider.** If there is a special model that is only able to work with a specific brick, we can always add a custom category to associate the model with the working brick.

*NOTE:* We need to clarify how to configure this when more than one container (and thus, more than one model) is defined within a brick.

## Brick
A brick is defined in the brick-list.yaml
```
- id: arduino:audio_classification
  name: Audio Classification
  category: audio, other_cat
  default_model_name: arduino:glass-breaking
```
The ```default_model_name``` field was added to specify a default model.

*NOTE:* What should be done if this is a custom model? The concept of a default custom model does not exist.

## App.yaml
This file contains the app's information and configuration.
```
name: Copy of Detect objects on images
description: Object detection in the browser
ports: []
bricks:
- arduino:web_ui: {}
- arduino:object_detection:
	model: dog-detector
- arduino:mood_detector: {}
icon: 🏞️
```
Here we added the model variable to be used for each brick.
*Note:* We assume here there is one model for each brick.

# This section will address Edge Impulse specific case
An Edge Impulse project is a set of input data and one or more impulses.

*NOTE:* It is unclear whether the project category can be defined at the project or impulse level.

An impulse is a set of configurations defined on input data that can generate a model according to a classification method, a processing block, and hardware parameters.

For each project, you can define one or more impulses. A model is the output of an impulse build, and a build can be executed more than one time, with different parameters.
Edge Impulse provides information about the last build present on the system.

*NOTE:* We need to discuss how to map the Edge Impulse structure to an AppLab model. Here are some notes for discussion:
* AppLab model is an EI project; AppLab build is the impulse. (We always have the last build of a specific impulse)
* AppLab model is an impulse; AppLab build is an Ei build.
* AppLab model is an impulse; AppLab build is a specific build with specific build parameters.

# Use Cases:
The following section explores whether this model is suitable for specific use cases.

## The user chooses a model for a brick (link Figma here)
In the brick configuration, there is a list of compatible models and the user selects the ```dog-detector``` model.
In this case, the model will be stored in ```App.yaml```. The brick section will look like:
```
bricks:
- arduino:web_ui: {}
- arduino:object_detection:
	model: dog-detector
- arduino:mood_detector: {}
```
## We need to know if a model build or a model definition can be deleted (link the Figma here):
To delete a build oir a model we need to know the state of the model in the system:
* **Downloaded** (present on the board and available for a brick to be used)
* **Installed** At least one brick is referencing the model.
* **In use** A brick instance is currently running with the model.

The Downloaded state can be checked by verifying the model's presence on the board's file system (FS).
The Installed state can be inferred by checking all application ```App.yaml``` files for references to the model.

*NOTE:* How is the "In Use" state determined?

We can provide a list of models, their states, and all the builds on the File System (FS) (if we implement different versions/builds).
We can allow the deletion of only specific builds or the entire model structure.

## We want to support a different model/executor
We aim to support a learning model, where the model is defined by a set of weights and generated by a framework like TensorFlow. We intend to deploy and run this model within a web browser using JavaScript (TensorFlow.js).

In this scenario, we can declare the model using its folder structure (API requests can be developed if required) and the necessary YAML files.

The model (which is the set of weights in this case) can either reside directly on the board or be available via a URL defined in its metadata. Since we assume the model runs in a browser, we need a brick where a container is configured to retrieve the model and execute the container's JavaScript.

The question here is how to make the model variables available to the container. There are several options:

1. Folder Mounting: In the brick configuration, we can specify the model identifier, and this ID can be passed to the container to mount the related folder. The container can then access all the model variables and proceed to deploy or download the model.

*NOTE:* Concurrency for different containers needs to be addressed here, but this is feasible.

2. Environment Variables: Alternatively, we can make the entire model configuration available to the container as environment variables.

# Questions:
1. Should we keep "audio" as a generic category instead of using something more specific, such as ```ei-audio``` or ```qualcomm-audio```?
At this level, this has no relevance.

Using the generic ```audio``` category allows us to make the same audio model compatible with bricks that use different AI execution engines.

If this becomes technically impossible in the future, we can then add a more specific category to map the model to its required brick.

2. What the "runner" variable represents? It is defined in the model.
   ```runner: brick```

3. Why there is the
```require model``` in the brick definition?

4. Check the use case of the Figma where we go to EI and come back with a model

# Changes from the current model/brick definition
Put in this section changes to brick, model and app configuration files.

