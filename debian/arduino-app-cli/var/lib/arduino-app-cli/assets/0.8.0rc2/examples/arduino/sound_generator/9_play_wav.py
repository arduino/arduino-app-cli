# SPDX-FileCopyrightText: Copyright (C) ARDUINO SRL (http://www.arduino.cc)
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME: Play a WAV file
from arduino.app_bricks.sound_generator import SoundGenerator
from arduino.app_utils import App

player = SoundGenerator()

# Provide the path to a WAV file in the app directory (e.g., "audio/sample.wav")
player.play_wav("audio/sample.wav", block=True)

App.run()
