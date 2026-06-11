# SPDX-FileCopyrightText: Copyright (C) Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

# EXAMPLE_NAME = "Synthesize speech audio from text as raw PCM bytes"

from arduino.app_bricks.tts import TextToSpeech

tts = TextToSpeech()

pcm = tts.synthesize_pcm("Hello, Arduino world!")
with open("synthesized_speech.pcm", "wb") as f:
    f.write(pcm)
