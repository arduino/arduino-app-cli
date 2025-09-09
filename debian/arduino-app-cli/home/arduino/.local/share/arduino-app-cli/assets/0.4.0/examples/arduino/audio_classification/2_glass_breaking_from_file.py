# SPDX-FileCopyrightText: Copyright (C) 2025 ARDUINO SA <http://www.arduino.cc>
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Detect the glass breaking sound from audio file"
# EXAMPLE_REQUIRES = "Requires an audio file with the glass breaking sound."
from arduino.app_bricks.audio_classification import AudioClassification

classifier = AudioClassification()

classification = classifier.classify_from_file("glass_breaking.wav")
print("Result:", classification)
