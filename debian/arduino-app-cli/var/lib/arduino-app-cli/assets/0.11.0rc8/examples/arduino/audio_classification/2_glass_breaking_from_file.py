# SPDX-FileCopyrightText: Copyright (C) Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Detect the glass breaking sound from audio file"
# EXAMPLE_REQUIRES = "Requires an audio file with the glass breaking sound."
from arduino.app_bricks.audio_classification import AudioClassification

classification = AudioClassification.classify_from_file("glass_breaking.wav")
print("Result:", classification)
