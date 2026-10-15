# SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
#
# SPDX-License-Identifier: MPL-2.0

# The brick brings the MCU code: the app has no sketch of its own.

import time

from arduino.app_utils import App

from led_matrix import COLS, MAX_BRIGHTNESS, ROWS, LedMatrix

matrix = LedMatrix()
column = 0
greeted = False


def loop():
    global column, greeted

    # The first call waits for the bridge, so the greeting goes here, not at import.
    if not greeted:
        matrix.print("UNO Q")
        greeted = True

    frame = bytearray(ROWS * COLS)
    for row in range(ROWS):
        frame[row * COLS + column] = MAX_BRIGHTNESS
    matrix.draw(frame)

    column = (column + 1) % COLS
    time.sleep(0.1)


App.run(user_loop=loop)
